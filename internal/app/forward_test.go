package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestForwardSetsAuthorizationHeader(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if auth != "Bearer token" {
		t.Fatalf("unexpected auth header: %q", auth)
	}
}

func TestForwardContentTypeJSON(t *testing.T) {
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("unexpected content-type: %q", contentType)
	}
}

func TestForwardBodySlimmed(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	raw := []byte(`{
		"action": {
			"type": "updateCard",
			"data": {
				"card": {"id": "69ae188a", "name": "[AVM Module Issue]: ...", "desc": "ignored"},
				"listBefore": {"name": "Backlog", "id": "x"},
				"listAfter": {"name": "Analyze", "id": "y"},
				"board": {"name": "Main Board"}
			},
			"memberCreator": {"fullName": "HeZijie", "username": "hzj"},
			"id": "action-id"
		},
		"model": "should-not-forward"
	}`)
	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), raw)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal forwarded body: %v", err)
	}

	action, ok := got["action"].(map[string]any)
	if !ok {
		t.Fatalf("missing action: %v", got)
	}
	if action["type"] != "updateCard" {
		t.Fatalf("unexpected action.type: %v", action)
	}

	data, ok := action["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing action.data: %v", action)
	}
	card, ok := data["card"].(map[string]any)
	if !ok || card["id"] != "69ae188a" {
		t.Fatalf("unexpected card: %v", data["card"])
	}
	if _, ok := card["name"]; ok {
		t.Fatalf("card.name should be removed: %v", card)
	}
	if _, ok := card["desc"]; ok {
		t.Fatalf("card.desc should be removed: %v", card)
	}

	listBefore, ok := data["listBefore"].(map[string]any)
	if !ok || listBefore["name"] != "Backlog" {
		t.Fatalf("unexpected listBefore: %v", data["listBefore"])
	}
	if listBefore["id"] != "x" {
		t.Fatalf("listBefore.id should be preserved: %v", listBefore)
	}

	listAfter, ok := data["listAfter"].(map[string]any)
	if !ok || listAfter["name"] != "Analyze" {
		t.Fatalf("unexpected listAfter: %v", data["listAfter"])
	}
	if listAfter["id"] != "y" {
		t.Fatalf("listAfter.id should be preserved: %v", listAfter)
	}
	if _, ok := data["board"]; ok {
		t.Fatalf("action.data.board should be removed: %v", data)
	}

	if _, ok := action["memberCreator"]; ok {
		t.Fatalf("memberCreator should be removed: %v", action)
	}
	if _, ok := action["id"]; ok {
		t.Fatalf("action.id should be removed: %v", action)
	}
	if _, ok := got["model"]; ok {
		t.Fatalf("top-level model should be removed: %v", got)
	}
}

func TestForwardRejectsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	_, _, err := f.Forward(context.Background(), []byte(`not-json`))
	if err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestForwardPropagatesResponseStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: time.Second})
	status, respBody, err := f.Forward(context.Background(), []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", status)
	}
	if string(respBody) != "accepted" {
		t.Fatalf("unexpected response body: %s", string(respBody))
	}
}

func TestForwardTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "token", &http.Client{Timeout: 10 * time.Millisecond})
	_, _, err := f.Forward(context.Background(), []byte(`{"k":"v"}`))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
