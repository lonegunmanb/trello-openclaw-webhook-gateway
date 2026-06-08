package localboard

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPCreateMoveAndCommentDispatchTrelloPayloads(t *testing.T) {
	store, err := Open(context.Background(), Options{DBPath: filepath.Join(t.TempDir(), ".jjc", "local-board.sqlite")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var payloads [][]byte
	handler := NewHandler(store, DispatchFunc(func(_ context.Context, raw []byte) error {
		payloads = append(payloads, append([]byte(nil), raw...))
		return nil
	}))

	createResp := doJSON(t, handler, http.MethodPost, "/api/cards", map[string]any{
		"title":       "AzureRM bug #77",
		"description": "https://github.com/hashicorp/terraform-provider-azurerm/issues/77\n\nbody",
	})
	cardID := nestedResponseString(t, createResp, "card", "id")
	if cardID == "" {
		t.Fatal("create response did not include card.id")
	}
	assertNestedString(t, payloads[0], "createCard", "action", "type")
	assertNestedString(t, payloads[0], "plan", "action", "data", "list", "id")

	doJSON(t, handler, http.MethodPost, "/api/cards/"+cardID+"/move", map[string]any{"to": "action"})
	assertNestedString(t, payloads[1], "updateCard", "action", "type")
	assertNestedString(t, payloads[1], "plan", "action", "data", "listBefore", "id")
	assertNestedString(t, payloads[1], "action", "action", "data", "listAfter", "id")

	doJSON(t, handler, http.MethodPost, "/api/cards/"+cardID+"/comments", map[string]any{"text": "ship it"})
	assertNestedString(t, payloads[2], "commentCard", "action", "type")
	assertNestedString(t, payloads[2], "ship it", "action", "data", "text")

	state := doJSON(t, handler, http.MethodGet, "/api/state", nil)
	cards, ok := state["cards"].([]any)
	if !ok || len(cards) != 1 {
		t.Fatalf("expected one card in state, got %#v", state["cards"])
	}
}

func TestHTTPStateReturnsEmptyCardsArray(t *testing.T) {
	store, err := Open(context.Background(), Options{DBPath: filepath.Join(t.TempDir(), ".jjc", "local-board.sqlite")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	state := doJSON(t, NewHandler(store, nil), http.MethodGet, "/api/state", nil)
	cards, ok := state["cards"].([]any)
	if !ok {
		t.Fatalf("cards should be an empty JSON array, got %#v", state["cards"])
	}
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %#v", cards)
	}
}

func TestHTTPStateReturnsEmptyCommentsArrayForCards(t *testing.T) {
	store, err := Open(context.Background(), Options{DBPath: filepath.Join(t.TempDir(), ".jjc", "local-board.sqlite")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	handler := NewHandler(store, nil)

	doJSON(t, handler, http.MethodPost, "/api/cards", map[string]any{
		"title":       "No comments yet",
		"description": "https://github.com/hashicorp/terraform-provider-azurerm/issues/88",
	})
	state := doJSON(t, handler, http.MethodGet, "/api/state", nil)
	cards := state["cards"].([]any)
	card := cards[0].(map[string]any)
	comments, ok := card["comments"].([]any)
	if !ok {
		t.Fatalf("card comments should be an empty JSON array, got %#v", card["comments"])
	}
	if len(comments) != 0 {
		t.Fatalf("expected no comments, got %#v", comments)
	}
}

func TestIndexHTMLSkipsNoopMoveDrop(t *testing.T) {
	for _, want := range []string{
		`draggingFrom = c.idList`,
		`if (draggingFrom === col.id) return`,
	} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("indexHTML missing no-op drag guard %q", want)
		}
	}
}

func TestIndexHTMLRefreshesWhenSSEReconnects(t *testing.T) {
	for _, want := range []string{
		`es.onopen = () => {`,
		`refresh();`,
	} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("indexHTML missing SSE reconnect refresh fragment %q", want)
		}
	}
}

func TestEventsStreamSendsHeartbeatWhileIdle(t *testing.T) {
	oldInterval := sseHeartbeatInterval
	sseHeartbeatInterval = time.Millisecond
	t.Cleanup(func() { sseHeartbeatInterval = oldInterval })

	store, err := Open(context.Background(), Options{DBPath: filepath.Join(t.TempDir(), ".jjc", "local-board.sqlite")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := httptest.NewServer(NewHandler(store, nil))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for i := 0; i < 4; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line %d: %v", i, err)
		}
		if strings.TrimSpace(line) == ": heartbeat" {
			return
		}
	}
	t.Fatal("SSE stream did not send heartbeat while idle")
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code < 200 || rr.Code >= 300 {
		t.Fatalf("%s %s status=%d body=%s", method, path, rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rr.Body.String())
	}
	return out
}

func nestedResponseString(t *testing.T, root map[string]any, path ...string) string {
	t.Helper()
	current := any(root)
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("response path %v reached non-object %T", path, current)
		}
		current = obj[key]
	}
	got, _ := current.(string)
	return got
}
