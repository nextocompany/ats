package peoplesoft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/clientcredentials"

	"github.com/nexto/hr-ats/pkg/config"
)

const (
	restMaxAttempts = 3
	restTimeout     = 30 * time.Second
)

// restClient calls the PeopleSoft Integration Broker over OAuth2 (client
// credentials) with retry + backoff.
type restClient struct {
	baseURL string
	http    *http.Client
}

func newRESTClient(cfg *config.Config) restClient {
	cc := &clientcredentials.Config{
		ClientID:     cfg.PSIBClientID,
		ClientSecret: cfg.PSIBClientSecret,
		TokenURL:     cfg.PSIBTokenURL,
	}
	httpClient := cc.Client(context.Background())
	httpClient.Timeout = restTimeout
	return restClient{baseURL: strings.TrimRight(cfg.PSIBBaseURL, "/"), http: httpClient}
}

func (c restClient) SyncHired(ctx context.Context, a Applicant) error {
	body, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("peoplesoft: marshal: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= restMaxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/applicant", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("peoplesoft: request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("peoplesoft: status %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second): // exponential-ish backoff
		}
	}
	return fmt.Errorf("peoplesoft: sync failed after %d attempts: %w", restMaxAttempts, lastErr)
}
