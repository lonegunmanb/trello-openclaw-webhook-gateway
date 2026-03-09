package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestBuildMessageMoveCard(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"action": map[string]any{
			"type": "updateCard",
			"data": map[string]any{
				"card":       map[string]any{"name": "Fix bug"},
				"listBefore": map[string]any{"name": "Ready for review"},
				"listAfter":  map[string]any{"name": "Approved for action"},
			},
			"memberCreator": map[string]any{"fullName": "Roger"},
		},
	})

	got := BuildMessage(raw)
	want := `Trello: 卡片 "Fix bug" 从 "Ready for review" 移到 "Approved for action" (by Roger)`
	if got != want {
		t.Fatalf("unexpected message:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestBuildMessageCommentCard(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"action": map[string]any{
			"type": "commentCard",
			"data": map[string]any{
				"text": "Looks good",
				"card": map[string]any{"name": "Implement feature"},
			},
			"memberCreator": map[string]any{"fullName": "Alice"},
		},
	})

	got := BuildMessage(raw)
	want := `Trello: Alice 在卡片 "Implement feature" 上评论: Looks good`
	if got != want {
		t.Fatalf("unexpected message:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestBuildMessageFallbackIncludesRawJSON(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"action": map[string]any{
			"type": "createCard",
			"data": map[string]any{
				"card":  map[string]any{"name": "New card"},
				"board": map[string]any{"name": "Main Board"},
			},
			"memberCreator": map[string]any{"fullName": "Bob"},
		},
	})

	got := BuildMessage(raw)
	if !strings.Contains(got, `Trello: createCard on card "New card" in board "Main Board" by Bob`) {
		t.Fatalf("fallback summary missing: %s", got)
	}
	if !strings.Contains(got, `"type":"createCard"`) {
		t.Fatalf("raw json missing: %s", got)
	}
}

func TestBuildMessageHandlesMissingFields(t *testing.T) {
	raw := []byte(`{"action":{"type":"updateCard"}}`)
	got := BuildMessage(raw)
	if got == "" {
		t.Fatal("message should not be empty")
	}
}
