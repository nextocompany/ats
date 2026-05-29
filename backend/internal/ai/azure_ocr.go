package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	docIntelAPIVersion = "2024-11-30"
	docIntelModel      = "prebuilt-layout"
	docIntelPollEvery  = 1 * time.Second
	docIntelPollMax    = 60
)

// azureOCR calls Azure AI Document Intelligence (prebuilt-layout) over REST.
type azureOCR struct {
	endpoint string
	key      string
	http     *http.Client
}

// NewAzureOCR builds the Document Intelligence client.
func NewAzureOCR(endpoint, key string) OCR {
	return azureOCR{
		endpoint: strings.TrimRight(endpoint, "/"),
		key:      key,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (a azureOCR) Extract(ctx context.Context, file []byte, _ string) (OCRResult, error) {
	analyzeURL := fmt.Sprintf(
		"%s/documentintelligence/documentModels/%s:analyze?api-version=%s&outputContentFormat=markdown",
		a.endpoint, docIntelModel, docIntelAPIVersion,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, analyzeURL, bytes.NewReader(file))
	if err != nil {
		return OCRResult{}, fmt.Errorf("ai: doc-intel request: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", a.key)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := a.http.Do(req)
	if err != nil {
		return OCRResult{}, fmt.Errorf("ai: doc-intel analyze: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		return OCRResult{}, fmt.Errorf("ai: doc-intel analyze status %d", resp.StatusCode)
	}
	opLoc := resp.Header.Get("Operation-Location")
	if opLoc == "" {
		return OCRResult{}, fmt.Errorf("ai: doc-intel missing Operation-Location")
	}

	return a.poll(ctx, opLoc)
}

type docIntelResult struct {
	Status        string `json:"status"`
	AnalyzeResult struct {
		Content string `json:"content"`
		Pages   []struct {
			Words []struct {
				Confidence float64 `json:"confidence"`
			} `json:"words"`
		} `json:"pages"`
	} `json:"analyzeResult"`
}

func (a azureOCR) poll(ctx context.Context, opLoc string) (OCRResult, error) {
	for attempt := 0; attempt < docIntelPollMax; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, opLoc, nil)
		if err != nil {
			return OCRResult{}, fmt.Errorf("ai: doc-intel poll request: %w", err)
		}
		req.Header.Set("Ocp-Apim-Subscription-Key", a.key)

		resp, err := a.http.Do(req)
		if err != nil {
			return OCRResult{}, fmt.Errorf("ai: doc-intel poll: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var r docIntelResult
		if err := json.Unmarshal(body, &r); err != nil {
			return OCRResult{}, fmt.Errorf("ai: doc-intel decode: %w", err)
		}
		switch r.Status {
		case "succeeded":
			return OCRResult{Text: r.AnalyzeResult.Content, Confidence: averageConfidence(r)}, nil
		case "failed":
			return OCRResult{}, fmt.Errorf("ai: doc-intel analysis failed")
		}

		select {
		case <-ctx.Done():
			return OCRResult{}, ctx.Err()
		case <-time.After(docIntelPollEvery):
		}
	}
	return OCRResult{}, fmt.Errorf("ai: doc-intel poll timed out")
}

func averageConfidence(r docIntelResult) float64 {
	var sum float64
	var n int
	for _, p := range r.AnalyzeResult.Pages {
		for _, w := range p.Words {
			sum += w.Confidence
			n++
		}
	}
	if n == 0 {
		return 1.0
	}
	return sum / float64(n)
}
