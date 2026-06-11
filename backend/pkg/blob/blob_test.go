package blob

import (
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// Azurite's well-known dev account — credentials only, no network is touched
// here (the client is constructed lazily and SAS signing is local crypto).
const devConnString = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;" +
	"AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;" +
	"BlobEndpoint=http://azurite:10000/devstoreaccount1;"

const devAccountName = "devstoreaccount1"
const devAccountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// newTestClient builds a Client without Connect() so no container call hits the
// network. SignedURL only needs the azblob client (for URL building) and the
// shared-key credential (for signing).
func newTestClient(t *testing.T, publicBase string) *Client {
	t.Helper()
	c, err := azblob.NewClientFromConnectionString(devConnString, nil)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	cred, err := azblob.NewSharedKeyCredential(devAccountName, devAccountKey)
	if err != nil {
		t.Fatalf("cred: %v", err)
	}
	return &Client{client: c, container: "resumes", cred: cred, publicBase: publicBase}
}

func TestParseAccount(t *testing.T) {
	name, key := parseAccount(devConnString)
	if name != devAccountName {
		t.Errorf("name = %q, want %q", name, devAccountName)
	}
	if key != devAccountKey {
		t.Errorf("key = %q, want %q", key, devAccountKey)
	}

	// Missing key (managed-identity style) → empty, so SignedURL can fall back.
	n2, k2 := parseAccount("BlobEndpoint=https://acct.blob.core.windows.net;")
	if n2 != "" || k2 != "" {
		t.Errorf("expected empty name/key, got %q/%q", n2, k2)
	}
}

func TestSignedURL_PinsVersionAndReadOnly(t *testing.T) {
	c := newTestClient(t, "")
	url, err := c.SignedURL("resume-abc.html", 15*time.Minute)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	for _, want := range []string{"sv=2021-08-06", "sig=", "sp=r", "sr=b"} {
		if !strings.Contains(url, want) {
			t.Errorf("signed url missing %q: %s", want, url)
		}
	}
	// No public endpoint → keeps the internal host.
	if !strings.HasPrefix(url, "http://azurite:10000/devstoreaccount1/resumes/resume-abc.html?") {
		t.Errorf("unexpected url prefix: %s", url)
	}
}

func TestSignedURL_RewritesPublicEndpoint(t *testing.T) {
	c := newTestClient(t, "http://localhost:10000/devstoreaccount1")
	url, err := c.SignedURL("resume-abc.html", 15*time.Minute)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	if !strings.HasPrefix(url, "http://localhost:10000/devstoreaccount1/resumes/resume-abc.html?") {
		t.Errorf("public host not rewritten: %s", url)
	}
	if strings.Contains(url, "azurite:10000") {
		t.Errorf("internal host leaked into public url: %s", url)
	}
	// The SAS query must survive the host rewrite intact.
	if !strings.Contains(url, "sv=2021-08-06") || !strings.Contains(url, "sig=") {
		t.Errorf("SAS query lost during rewrite: %s", url)
	}
}

func TestToPublic_NoBaseIsNoop(t *testing.T) {
	c := newTestClient(t, "")
	in := "http://azurite:10000/devstoreaccount1/resumes/x.html?sig=abc"
	if got := c.toPublic(in); got != in {
		t.Errorf("toPublic mutated url without publicBase: %s", got)
	}
}
