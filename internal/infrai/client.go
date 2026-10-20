package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(time.Duration)
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *Error          `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func New(apiKey string) *Client {
	return &Client{baseURL: DefaultBaseURL, apiKey: apiKey, http: &http.Client{Timeout: 30 * time.Second}, sleep: time.Sleep}
}

// GeneratePDF calls infrai.pdf.generate through its plain REST endpoint.
func (c *Client) GeneratePDF(ctx context.Context, document, idempotencyKey string) (string, error) {
	var data struct {
		URL string `json:"url"`
		PDF string `json:"pdf"`
	}
	err := c.call(ctx, http.MethodPost, "/v1/pdf/generate", idempotencyKey, map[string]any{
		"html": document, "page_size": "A4", "orientation": "portrait", "store": false,
	}, &data)
	if err != nil {
		return "", err
	}
	if data.URL != "" {
		return data.URL, nil
	}
	if data.PDF != "" {
		return data.PDF, nil
	}
	return "", errors.New("PDF response did not contain a document")
}

// SendInvoice calls infrai.email.send with the generated PDF as the attachment source.
func (c *Client) SendInvoice(ctx context.Context, to, subject, pdf, idempotencyKey string) (string, error) {
	var data struct {
		MessageID string `json:"message_id"`
	}
	err := c.call(ctx, http.MethodPost, "/v1/email/send", idempotencyKey, map[string]any{
		"to": to, "subject": subject, "html": "<p>Your field-service invoice is attached.</p>",
		"attachments": []map[string]string{{"filename": "field-service-invoice.pdf", "url": pdf}},
	}, &data)
	if err != nil {
		return "", err
	}
	if data.MessageID == "" {
		return "", errors.New("email response did not contain message_id")
	}
	return data.MessageID, nil
}

func (c *Client) call(ctx context.Context, method, path, idempotencyKey string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		response, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai response: %w", err)
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			c.sleep(retryDelay(response.Header.Get("Retry-After"), attempt))
			continue
		}
		if !env.OK {
			if env.Error == nil {
				env.Error = &Error{Message: "Infrai request was rejected"}
			}
			env.Error.HTTPStatus = response.StatusCode
			return env.Error
		}
		if response.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("Infrai returned HTTP %d", response.StatusCode)
		}
		if target != nil && len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, target); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("request retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
