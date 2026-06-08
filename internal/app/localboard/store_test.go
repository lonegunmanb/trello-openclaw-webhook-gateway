package localboard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lonegunmanb/jjc/internal/app/trelloclient"
)

func TestOpenCreatesSQLiteDatabaseUnderWorkspace(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".jjc", "local-board.sqlite")

	store, err := Open(context.Background(), Options{DBPath: dbPath})
	if err != nil {
		t.Fatalf("open local board store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite database at %s: %v", dbPath, err)
	}
}

func TestStorePersistsCardAndImplementsTrelloClientReads(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	card, err := store.CreateCard(ctx, CreateCardInput{
		Name:   "AzureRM issue #123",
		Desc:   "https://github.com/hashicorp/terraform-provider-azurerm/issues/123\n\nNeeds triage.",
		ListID: "plan",
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	got, err := store.GetCard(ctx, card.ID)
	if err != nil {
		t.Fatalf("get card: %v", err)
	}
	if got.Name != "AzureRM issue #123" {
		t.Fatalf("unexpected card name: %q", got.Name)
	}
	if got.FirstLine != "https://github.com/hashicorp/terraform-provider-azurerm/issues/123" {
		t.Fatalf("unexpected first line: %q", got.FirstLine)
	}
	if got.IDList != "plan" || got.IDBoard != DefaultBoardID {
		t.Fatalf("unexpected list/board: %q/%q", got.IDList, got.IDBoard)
	}

	list, err := store.GetCardList(ctx, card.ID)
	if err != nil {
		t.Fatalf("get card list: %v", err)
	}
	if list.ID != "plan" || list.Name != "Analyze" {
		t.Fatalf("unexpected list: %#v", list)
	}

	var _ trelloclient.Client = store
}

func TestStoreCommentsAreReturnedOldestFirstAndLatest(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	card, err := store.CreateCard(ctx, CreateCardInput{Name: "Card", Desc: "https://github.com/hashicorp/terraform-provider-azurerm/issues/1", ListID: "plan"})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	first, err := store.AddComment(ctx, card.ID, "first")
	if err != nil {
		t.Fatalf("add first comment: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := store.AddComment(ctx, card.ID, "second")
	if err != nil {
		t.Fatalf("add second comment: %v", err)
	}

	latest, err := store.GetLatestComment(ctx, card.ID)
	if err != nil {
		t.Fatalf("latest comment: %v", err)
	}
	if latest.ID != second.ID || latest.Text != "second" {
		t.Fatalf("unexpected latest comment: %#v", latest)
	}

	comments, err := store.ListCommentsSince(ctx, card.ID, time.Time{})
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(comments) != 2 || comments[0].ID != first.ID || comments[1].ID != second.ID {
		t.Fatalf("comments not oldest-first: %#v", comments)
	}
}

func TestMoveAndCommentEventsUseTrelloWebhookShape(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	card, err := store.CreateCard(ctx, CreateCardInput{Name: "Card", Desc: "https://github.com/hashicorp/terraform-provider-azurerm/issues/1", ListID: "plan"})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	payload, err := BuildMovePayload(card, ColumnByID["plan"], ColumnByID["action"])
	if err != nil {
		t.Fatalf("build move payload: %v", err)
	}
	assertNestedString(t, payload, "updateCard", "action", "type")
	assertNestedString(t, payload, card.ID, "action", "data", "card", "id")
	assertNestedString(t, payload, "plan", "action", "data", "listBefore", "id")
	assertNestedString(t, payload, "Analyze", "action", "data", "listBefore", "name")
	assertNestedString(t, payload, "action", "action", "data", "listAfter", "id")
	assertNestedString(t, payload, "In action", "action", "data", "listAfter", "name")

	comment, err := store.AddHumanComment(ctx, card.ID, "please continue")
	if err != nil {
		t.Fatalf("add human comment: %v", err)
	}
	commentPayload, err := BuildCommentPayload(card, comment)
	if err != nil {
		t.Fatalf("build comment payload: %v", err)
	}
	assertNestedString(t, commentPayload, "commentCard", "action", "type")
	assertNestedString(t, commentPayload, "please continue", "action", "data", "text")
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), Options{DBPath: filepath.Join(t.TempDir(), ".jjc", "local-board.sqlite")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func assertNestedString(t *testing.T, raw []byte, want string, path ...string) {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("payload is not json: %v\n%s", err, raw)
	}
	current := any(root)
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s: got non-object %T", strings.Join(path, "."), current)
		}
		current = obj[key]
	}
	if got, ok := current.(string); !ok || got != want {
		t.Fatalf("%s: got %#v want %q", strings.Join(path, "."), current, want)
	}
}
