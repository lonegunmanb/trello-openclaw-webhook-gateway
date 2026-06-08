package localboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var sseHeartbeatInterval = 15 * time.Second

type DispatchFunc func(ctx context.Context, raw []byte) error

type handler struct {
	store    *Store
	dispatch DispatchFunc

	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func NewHandler(store *Store, dispatch DispatchFunc) http.Handler {
	h := &handler{store: store, dispatch: dispatch, clients: map[chan struct{}]struct{}{}}
	store.SetNotify(h.broadcast)
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/events", h.handleEvents)
	mux.HandleFunc("/api/state", h.handleState)
	mux.HandleFunc("/api/cards", h.handleCards)
	mux.HandleFunc("/api/cards/", h.handleCardSubresource)
	return mux
}

func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/" {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("content-type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (h *handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	w.Header().Set("connection", "keep-alive")
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()
	_, _ = fmt.Fprint(w, ": ok\n\n")
	flusher.Flush()
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-ch:
			_, _ = fmt.Fprint(w, "data: {}\n\n")
			flusher.Flush()
		}
	}
}

func (h *handler) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cards, err := h.store.ListCards(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"columns": Columns, "cards": cards})
}

func (h *handler) handleCards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &body); err != nil {
		h.writeError(w, err)
		return
	}
	card, err := h.store.CreateCard(r.Context(), CreateCardInput{Name: body.Title, Desc: body.Description, ListID: "plan"})
	if err != nil {
		h.writeError(w, err)
		return
	}
	payload, err := BuildCreatePayload(card, ColumnByID[card.IDList])
	if err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.dispatchPayload(r.Context(), payload); err != nil {
		h.writeError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"card": card})
}

func (h *handler) handleCardSubresource(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/cards/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	cardID := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch {
	case r.Method == http.MethodGet && sub == "":
		card, err := h.store.GetCard(r.Context(), cardID)
		if err != nil {
			h.writeError(w, err)
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]any{"card": card})
	case r.Method == http.MethodPost && sub == "move":
		h.handleMove(w, r, cardID)
	case r.Method == http.MethodPost && sub == "comments":
		h.handleComment(w, r, cardID)
	default:
		h.writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *handler) handleMove(w http.ResponseWriter, r *http.Request, cardID string) {
	var body struct {
		To string `json:"to"`
	}
	if err := readJSON(r, &body); err != nil {
		h.writeError(w, err)
		return
	}
	before, err := h.store.GetCard(r.Context(), cardID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	from, ok := ColumnByID[before.IDList]
	if !ok {
		h.writeError(w, fmt.Errorf("localboard: unknown source list %q", before.IDList))
		return
	}
	to, ok := ColumnByID[body.To]
	if !ok {
		h.writeError(w, fmt.Errorf("localboard: unknown target list %q", body.To))
		return
	}
	if err := h.store.MoveCard(r.Context(), cardID, body.To); err != nil {
		h.writeError(w, err)
		return
	}
	after, err := h.store.GetCard(r.Context(), cardID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	payload, err := BuildMovePayload(after, from, to)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.dispatchPayload(r.Context(), payload); err != nil {
		h.writeError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"card": after})
}

func (h *handler) handleComment(w http.ResponseWriter, r *http.Request, cardID string) {
	var body struct {
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil {
		h.writeError(w, err)
		return
	}
	card, err := h.store.GetCard(r.Context(), cardID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	comment, err := h.store.AddHumanComment(r.Context(), cardID, body.Text)
	if err != nil {
		h.writeError(w, err)
		return
	}
	payload, err := BuildCommentPayload(card, comment)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.dispatchPayload(r.Context(), payload); err != nil {
		h.writeError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"comment": comment})
}

func (h *handler) dispatchPayload(ctx context.Context, payload []byte) error {
	if h.dispatch == nil {
		return nil
	}
	return h.dispatch(ctx, payload)
}

func (h *handler) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (h *handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, context.Canceled) {
		status = http.StatusRequestTimeout
	}
	h.writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (h *handler) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("localboard: invalid json body: %w", err)
	}
	return nil
}
