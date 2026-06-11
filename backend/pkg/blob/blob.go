// Package blob owns the Azure Blob Storage client (Azurite in local dev). It
// ensures the resumes container exists on boot and exposes a health check.
package blob

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// sasServiceVersion pins the SAS signing version. The SDK otherwise defaults to
// its newest service version, which Azurite (local dev) cannot validate →
// "Server failed to authenticate the request" (403). 2021-08-06 is supported by
// both Azurite and real Azure, so resume links work everywhere.
const sasServiceVersion = "2021-08-06"

// Client wraps the azblob client together with the working container name.
type Client struct {
	client    *azblob.Client
	container string
	// cred is the shared-key credential parsed from the connection string, used
	// to sign SAS URLs with a pinned version. Nil when the connection string has
	// no account key (e.g. managed-identity auth) — SignedURL then falls back to
	// the SDK default.
	cred *azblob.SharedKeyCredential
	// publicBase, when set (BLOB_PUBLIC_ENDPOINT), replaces the internal service
	// endpoint in signed URLs. Local dev needs this: the API reaches Azurite at
	// http://azurite:10000 (a Docker-internal host the browser cannot resolve),
	// but the SAS is path-based, so the browser-facing link can point at
	// http://localhost:10000 instead. Unset in production → no rewrite.
	publicBase string
}

// Connect builds a client from the connection string and ensures the container
// exists (create-if-not-exists is idempotent).
func Connect(ctx context.Context, connString, containerName string) (*Client, error) {
	c, err := azblob.NewClientFromConnectionString(connString, nil)
	if err != nil {
		return nil, fmt.Errorf("blob: client: %w", err)
	}
	bc := &Client{
		client:     c,
		container:  containerName,
		publicBase: strings.TrimRight(os.Getenv("BLOB_PUBLIC_ENDPOINT"), "/"),
	}
	// Best-effort: derive a shared-key credential for version-pinned SAS signing.
	if name, key := parseAccount(connString); name != "" && key != "" {
		if cred, credErr := azblob.NewSharedKeyCredential(name, key); credErr == nil {
			bc.cred = cred
		}
	}
	if err := bc.ensureContainer(ctx); err != nil {
		return nil, err
	}
	return bc, nil
}

// parseAccount extracts AccountName and AccountKey from a storage connection
// string (Key=Value;Key=Value;...). Returns empty strings when absent.
func parseAccount(connString string) (name, key string) {
	for _, part := range strings.Split(connString, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "AccountName":
			name = strings.TrimSpace(kv[1])
		case "AccountKey":
			key = strings.TrimSpace(kv[1])
		}
	}
	return name, key
}

// ensureContainer creates the container, treating "already exists" as success.
func (c *Client) ensureContainer(ctx context.Context) error {
	_, err := c.client.CreateContainer(ctx, c.container, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("blob: create container %q: %w", c.container, err)
	}
	return nil
}

// HealthCheck confirms the storage endpoint is reachable and the container is
// listable by requesting the first page of blobs.
func (c *Client) HealthCheck(ctx context.Context) error {
	pager := c.client.NewListBlobsFlatPager(c.container, nil)
	if _, err := pager.NextPage(ctx); err != nil {
		return fmt.Errorf("blob: health: %w", err)
	}
	return nil
}

// Upload writes data to the named blob (overwriting if present) and returns the
// blob URL. Uploads are idempotent by name, which the pipeline relies on for
// safe task retries.
func (c *Client) Upload(ctx context.Context, name string, data []byte, contentType string) (string, error) {
	_, err := c.client.UploadBuffer(ctx, c.container, name, data, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: &contentType},
	})
	if err != nil {
		return "", fmt.Errorf("blob: upload %q: %w", name, err)
	}
	return strings.TrimRight(c.client.ServiceClient().URL(), "/") + "/" + c.container + "/" + name, nil
}

// SignedURL returns a short-lived, read-only SAS URL for the named blob. When a
// shared-key credential is available it signs with a pinned SAS version (so the
// link validates against Azurite as well as real Azure); otherwise it falls back
// to the SDK default.
func (c *Client) SignedURL(name string, ttl time.Duration) (string, error) {
	bc := c.client.ServiceClient().NewContainerClient(c.container).NewBlobClient(name)
	if c.cred == nil {
		url, err := bc.GetSASURL(sas.BlobPermissions{Read: true}, time.Now().Add(ttl), nil)
		if err != nil {
			return "", fmt.Errorf("blob: signed url for %q: %w", name, err)
		}
		return c.toPublic(url), nil
	}

	now := time.Now().UTC()
	vals := sas.BlobSignatureValues{
		Version:       sasServiceVersion,
		Protocol:      sas.ProtocolHTTPSandHTTP,
		StartTime:     now.Add(-5 * time.Minute),
		ExpiryTime:    now.Add(ttl),
		Permissions:   (&sas.BlobPermissions{Read: true}).String(),
		ContainerName: c.container,
		BlobName:      name,
	}
	qp, err := vals.SignWithSharedKey(c.cred)
	if err != nil {
		return "", fmt.Errorf("blob: signed url for %q: %w", name, err)
	}
	return c.toPublic(bc.URL()) + "?" + qp.Encode(), nil
}

// toPublic rewrites the internal service endpoint to the browser-facing one when
// BLOB_PUBLIC_ENDPOINT is configured (local dev). No-op in production.
func (c *Client) toPublic(rawURL string) string {
	if c.publicBase == "" {
		return rawURL
	}
	internal := strings.TrimRight(c.client.ServiceClient().URL(), "/")
	if strings.HasPrefix(rawURL, internal) {
		return c.publicBase + strings.TrimPrefix(rawURL, internal)
	}
	return rawURL
}

// SignedURLForStored derives the blob key from a previously stored full URL and
// returns a signed URL for it.
func (c *Client) SignedURLForStored(storedURL string, ttl time.Duration) (string, error) {
	marker := "/" + c.container + "/"
	i := strings.Index(storedURL, marker)
	if i < 0 {
		return "", fmt.Errorf("blob: cannot derive key from %q", storedURL)
	}
	return c.SignedURL(storedURL[i+len(marker):], ttl)
}

// Delete removes the named blob. A missing blob is treated as success so the
// retention sweep is idempotent across re-runs.
func (c *Client) Delete(ctx context.Context, name string) error {
	_, err := c.client.DeleteBlob(ctx, c.container, name, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.BlobNotFound) {
		return fmt.Errorf("blob: delete %q: %w", name, err)
	}
	return nil
}

// DeleteStored derives the blob key from a previously stored full URL and deletes
// it. Mirrors SignedURLForStored's key derivation.
func (c *Client) DeleteStored(ctx context.Context, storedURL string) error {
	marker := "/" + c.container + "/"
	i := strings.Index(storedURL, marker)
	if i < 0 {
		return fmt.Errorf("blob: cannot derive key from %q", storedURL)
	}
	return c.Delete(ctx, storedURL[i+len(marker):])
}

// Download reads the named blob into memory.
func (c *Client) Download(ctx context.Context, name string) ([]byte, error) {
	resp, err := c.client.DownloadStream(ctx, c.container, name, nil)
	if err != nil {
		return nil, fmt.Errorf("blob: download %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("blob: read %q: %w", name, err)
	}
	return buf.Bytes(), nil
}
