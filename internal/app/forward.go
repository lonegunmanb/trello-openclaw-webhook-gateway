package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Forwarder struct {
	forwardURL   string
	forwardToken string
	client       *http.Client
}

func NewForwarder(forwardURL, forwardToken string, client *http.Client) *Forwarder {
	return &Forwarder{forwardURL: forwardURL, forwardToken: forwardToken, client: client}
}

func (f *Forwarder) Forward(ctx context.Context, message string) (int, []byte, error) {
	payload := map[string]any{
		"message": message,
		"name":    "Trello",
		"deliver": true,
		"channel": "telegram",
		"to":      "399076135",
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal forward payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.forwardURL, bytes.NewReader(b))
	if err != nil {
		return 0, nil, fmt.Errorf("create forward request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.forwardToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("forward request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read forward response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}
