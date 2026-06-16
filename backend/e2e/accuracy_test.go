//go:build e2e

// Parse-accuracy harness. OPT-IN: set CVSET_DIR to a directory of real CVs, each
// with a matching <name>.expected.json ground-truth file. Uploads each via the bulk
// intake endpoint, waits for the pipeline, fetches the parsed profile blob, and
// scores it (scorecard.go). Requires the target stack to run REAL Azure AI — mocks
// produce fixed output and cannot validate accuracy.
//
//	CVSET_DIR=./e2e/cvset E2E_API_URL=https://<staging-api> DB_URL=<staging-db> \
//	  HR_COOKIE="hr_auth=..." MACRO_MIN=0.80 \
//	  go test -tags e2e -run Accuracy -timeout 30m ./e2e/...
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/ai"
)

func extContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return ""
	}
}

func floatEnv(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// bulkUploadOne uploads a single CV via the bulk-intake endpoint and returns the
// created application id.
func bulkUploadOne(t *testing.T, positionID uuid.UUID, data []byte, filename, cookie string) uuid.UUID {
	t.Helper()
	ct := extContentType(filename)
	if ct == "" {
		t.Fatalf("unsupported sample type: %s", filename)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("position_id", positionID.String())
	ph := textproto.MIMEHeader{}
	ph.Set("Content-Disposition", `form-data; name="resumes"; filename="`+filename+`"`)
	ph.Set("Content-Type", ct)
	fw, _ := w.CreatePart(ph)
	_, _ = fw.Write(data)
	_ = w.Close()

	req, _ := http.NewRequest(http.MethodPost, apiBase()+"/api/v1/applications/bulk-intake", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("bulk upload %s: %v", filename, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("bulk upload %s expected 200, got %d: %s", filename, resp.StatusCode, b)
	}
	var env struct {
		Data struct {
			Created []struct {
				ApplicationID uuid.UUID `json:"application_id"`
			} `json:"created"`
			Failed []struct {
				Filename string `json:"filename"`
				Error    string `json:"error"`
			} `json:"failed"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode bulk response: %v", err)
	}
	if len(env.Data.Created) != 1 {
		t.Fatalf("expected 1 created app for %s, got %d (failed: %+v)", filename, len(env.Data.Created), env.Data.Failed)
	}
	return env.Data.Created[0].ApplicationID
}

func fetchProfile(t *testing.T, url string) ai.Profile {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("fetch profile blob: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile blob status %d", resp.StatusCode)
	}
	var p ai.Profile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode profile blob: %v", err)
	}
	return p
}

func TestAccuracy(t *testing.T) {
	dir := os.Getenv("CVSET_DIR")
	if dir == "" {
		t.Skip("CVSET_DIR not set — skipping parse-accuracy harness (operator-run on staging)")
	}
	waitHealthy(t)
	pool := mustPool(t)
	positionID := seedPositionStore(t, pool, "ACC-001")
	cookie := os.Getenv("HR_COOKIE")

	matches, _ := filepath.Glob(filepath.Join(dir, "*.expected.json"))
	if len(matches) == 0 {
		t.Fatalf("no *.expected.json files in %s", dir)
	}

	var scores []CVScore
	for _, expPath := range matches {
		base := strings.TrimSuffix(filepath.Base(expPath), ".expected.json")
		cvPath := findCV(t, dir, base)
		data, err := os.ReadFile(cvPath)
		if err != nil {
			t.Fatalf("read cv %s: %v", cvPath, err)
		}
		expRaw, err := os.ReadFile(expPath)
		if err != nil {
			t.Fatalf("read expected %s: %v", expPath, err)
		}
		var exp Expected
		if err := json.Unmarshal(expRaw, &exp); err != nil {
			t.Fatalf("parse %s: %v", expPath, err)
		}

		appID := bulkUploadOne(t, positionID, data, filepath.Base(cvPath), cookie)

		var profileURL string
		var conf float64
		pollDB(t, 180*time.Second, func(ctx context.Context) bool {
			var status string
			err := pool.QueryRow(ctx,
				`SELECT status, COALESCE(parsed_profile_blob_url,''), COALESCE(ocr_confidence,0)
				 FROM applications WHERE id = $1`, appID,
			).Scan(&status, &profileURL, &conf)
			if err != nil {
				return false
			}
			return profileURL != "" && (status == "scored" || status == "rejected" || status == "parsed")
		})

		profile := fetchProfile(t, profileURL)
		scores = append(scores, Compare(filepath.Base(cvPath), conf, profile, exp))
	}

	agg := AggregateScores(scores)
	t.Logf("\n%s", agg.Format())

	macroMin := floatEnv("MACRO_MIN", 0.80)
	if agg.MacroAverage < macroMin {
		t.Fatalf("macro average %.3f below threshold %.3f", agg.MacroAverage, macroMin)
	}
}

// findCV locates the CV file for a base name (any supported extension).
func findCV(t *testing.T, dir, base string) string {
	t.Helper()
	for _, ext := range []string{".pdf", ".docx", ".png", ".jpg", ".jpeg"} {
		p := filepath.Join(dir, base+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatalf("no CV file for %q in %s (expected one of pdf/docx/png/jpg)", base, dir)
	return ""
}
