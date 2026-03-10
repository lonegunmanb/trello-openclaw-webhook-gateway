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

func (f *Forwarder) Forward(ctx context.Context, rawBody []byte) (int, []byte, error) {
	slimBody, err := slimRawBody(rawBody)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.forwardURL, bytes.NewReader(slimBody))
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

func slimRawBody(rawBody []byte) ([]byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return nil, fmt.Errorf("raw body must be valid json: %w", err)
	}

	action := map[string]any{}
	if v, ok := nestedString(raw, "action", "type"); ok {
		action["type"] = v
	}

	data := map[string]any{}
	card := map[string]any{}
	if v, ok := nestedString(raw, "action", "data", "card", "id"); ok {
		card["id"] = v
	}
	if len(card) > 0 {
		data["card"] = card
	}

	listBefore := map[string]any{}
	if v, ok := nestedString(raw, "action", "data", "listBefore", "name"); ok {
		listBefore["name"] = v
	}
	if len(listBefore) > 0 {
		data["listBefore"] = listBefore
	}

	listAfter := map[string]any{}
	if v, ok := nestedString(raw, "action", "data", "listAfter", "name"); ok {
		listAfter["name"] = v
	}
	if len(listAfter) > 0 {
		data["listAfter"] = listAfter
	}

	if len(data) > 0 {
		action["data"] = data
	}

	payload := map[string]any{}
	if len(action) > 0 {
		payload["action"] = action
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal slim payload: %w", err)
	}
	return b, nil
}

func nestedString(m map[string]any, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}

	current := any(m)
	for i, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		value, ok := obj[key]
		if !ok {
			return "", false
		}
		if i == len(path)-1 {
			s, ok := value.(string)
			if !ok || s == "" {
				return "", false
			}
			return s, true
		}
		current = value
	}

	return "", false
}
