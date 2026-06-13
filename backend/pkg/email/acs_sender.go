package email

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// acsSender delivers via the Azure Communication Services Email REST API. There
// is no official Go SDK, so requests are signed with the resource access key using
// ACS's shared-key HMAC scheme (identical to Azure Storage).
type acsSender struct {
	endpoint  string // https://<resource>.<region>.communication.azure.com
	accessKey string // base64 access key
	sender    string // verified sender address
	http      *http.Client
}

func newACSSender(endpoint, accessKey, sender string) *acsSender {
	return &acsSender{
		endpoint:  strings.TrimRight(endpoint, "/"),
		accessKey: accessKey,
		sender:    sender,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

// acsRecipient / acsContent / acsPayload model the ACS emails:send request body.
type acsRecipient struct {
	Address string `json:"address"`
}
type acsContent struct {
	Subject   string `json:"subject"`
	PlainText string `json:"plainText"`
	HTML      string `json:"html,omitempty"`
}
type acsPayload struct {
	SenderAddress string     `json:"senderAddress"`
	Content       acsContent `json:"content"`
	Recipients    struct {
		To []acsRecipient `json:"to"`
	} `json:"recipients"`
}

// Send POSTs the message to {endpoint}/emails:send. ACS accepts asynchronously
// (202 Accepted + Operation-Location); for OTP we only need "accepted", so we do
// not poll the operation status.
func (s *acsSender) Send(ctx context.Context, m Message) error {
	var p acsPayload
	p.SenderAddress = s.sender
	p.Content = acsContent{Subject: m.Subject, PlainText: m.PlainText, HTML: m.HTML}
	p.Recipients.To = []acsRecipient{{Address: m.To}}

	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	const pathAndQuery = "/emails:send?api-version=2023-03-31"
	u, err := url.Parse(s.endpoint + pathAndQuery)
	if err != nil {
		return fmt.Errorf("email: parse endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := signACS(req, s.accessKey, u, body, time.Now().UTC()); err != nil {
		return fmt.Errorf("email: sign: %w", err)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("email: acs status %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

// signACS applies the ACS shared-key HMAC signature to req. It sets x-ms-date,
// host, x-ms-content-sha256 and the Authorization header. Exposed (package-level,
// pure) so the signature is unit-testable with a fixed key/date/body.
func signACS(req *http.Request, accessKey string, u *url.URL, body []byte, now time.Time) error {
	key, err := base64.StdEncoding.DecodeString(accessKey)
	if err != nil {
		return fmt.Errorf("email: decode access key: %w", err)
	}
	dateStr := now.Format(http.TimeFormat) // RFC1123 GMT
	host := u.Host

	contentHash := sha256.Sum256(body)
	contentHashB64 := base64.StdEncoding.EncodeToString(contentHash[:])

	pathAndQuery := u.Path
	if u.RawQuery != "" {
		pathAndQuery += "?" + u.RawQuery
	}

	// StringToSign = VERB\n path-and-query \n x-ms-date;host;x-ms-content-sha256
	stringToSign := strings.Join([]string{
		req.Method,
		pathAndQuery,
		dateStr + ";" + host + ";" + contentHashB64,
	}, "\n")

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("x-ms-date", dateStr)
	req.Header.Set("x-ms-content-sha256", contentHashB64)
	req.Host = host
	req.Header.Set("Authorization", "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature="+sig)
	return nil
}
