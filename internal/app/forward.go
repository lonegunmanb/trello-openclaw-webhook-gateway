package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Forwarder struct {
	forwardURL   string
	forwardToken string
	model        string
	prompt       string
	client       *http.Client
}

func NewForwarder(forwardURL, forwardToken, model, prompt string, client *http.Client) *Forwarder {
	return &Forwarder{forwardURL: forwardURL, forwardToken: forwardToken, model: model, prompt: prompt, client: client}
}

func (f *Forwarder) Forward(ctx context.Context, message string, rawBody []byte) (int, []byte, error) {
	rawPayloadBase64 := base64.StdEncoding.EncodeToString(rawBody)
	fullMessage := f.prompt + "\n\n" + message + "\n\nRaw payload (base64):\n" + rawPayloadBase64

	payload := map[string]any{
		"message": fullMessage,
		"name":    "Trello",
		"deliver": false,
		"model":   f.model,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal forward payload: %w", err)
	}
	log.Printf("forward_payload=%s", string(b))

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
