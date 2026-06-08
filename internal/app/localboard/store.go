package localboard

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lonegunmanb/jjc/internal/app/trelloclient"
	_ "modernc.org/sqlite"
)

const (
	DefaultBoardID   = "local-board"
	DefaultBoardName = "Local JJC Board"
	AgentName        = "JJC Local Agent"
	HumanName        = "Local Operator"
)

type Options struct {
	DBPath string
}

type Column struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phase string `json:"phase"`
}

var Columns = []Column{
	{ID: "plan", Name: "Analyze", Phase: "plan"},
	{ID: "wait.plan_review", Name: "Ready for plan review", Phase: "wait"},
	{ID: "action", Name: "In action", Phase: "action"},
	{ID: "wait.action_review", Name: "Ready for review", Phase: "wait"},
	{ID: "wait.generic", Name: "Pending PR", Phase: "wait"},
	{ID: "wait.exception", Name: "Need Attention", Phase: "wait"},
	{ID: "done", Name: "Done", Phase: "done"},
}

var ColumnByID = func() map[string]Column {
	out := make(map[string]Column, len(Columns))
	for _, column := range Columns {
		out[column.ID] = column
	}
	return out
}()

type Store struct {
	db      *sql.DB
	boardID string
	now     func() time.Time

	notifyMu sync.Mutex
	notify   func()
}

type CardView struct {
	trelloclient.Card
	Comments []trelloclient.Comment `json:"comments"`
}

type CreateCardInput struct {
	Name   string
	Desc   string
	ListID string
}

func Open(ctx context.Context, opts Options) (*Store, error) {
	if strings.TrimSpace(opts.DBPath) == "" {
		return nil, errors.New("localboard: db path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(opts.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("localboard: create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", opts.DBPath)
	if err != nil {
		return nil, fmt.Errorf("localboard: open sqlite: %w", err)
	}
	store := &Store{db: db, boardID: DefaultBoardID, now: time.Now}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) SetNotify(fn func()) {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	s.notify = fn
}

func (s *Store) signalChange() {
	s.notifyMu.Lock()
	notify := s.notify
	s.notifyMu.Unlock()
	if notify != nil {
		notify()
	}
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS cards (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			desc TEXT NOT NULL,
			id_list TEXT NOT NULL,
			id_board TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			card_id TEXT NOT NULL,
			text TEXT NOT NULL,
			author TEXT NOT NULL,
			at TEXT NOT NULL,
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE CASCADE
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("localboard: migrate: %w", err)
		}
	}
	return nil
}

func (s *Store) CreateCard(ctx context.Context, in CreateCardInput) (trelloclient.Card, error) {
	name := strings.TrimSpace(in.Name)
	desc := strings.TrimSpace(in.Desc)
	listID := strings.TrimSpace(in.ListID)
	if desc == "" {
		return trelloclient.Card{}, errors.New("localboard: card desc is empty")
	}
	if listID == "" {
		listID = "plan"
	}
	if _, ok := ColumnByID[listID]; !ok {
		return trelloclient.Card{}, fmt.Errorf("localboard: unknown list id %q", listID)
	}
	if name == "" {
		name = firstNonEmptyLine(desc)
	}
	id := newID("c")
	now := s.now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cards (id, name, desc, id_list, id_board, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, desc, listID, s.boardID, now, now)
	if err != nil {
		return trelloclient.Card{}, fmt.Errorf("localboard: insert card: %w", err)
	}
	s.signalChange()
	return s.GetCard(ctx, id)
}

func (s *Store) GetCard(ctx context.Context, cardID string) (trelloclient.Card, error) {
	if strings.TrimSpace(cardID) == "" {
		return trelloclient.Card{}, errors.New("localboard: card id is empty")
	}
	var card trelloclient.Card
	err := s.db.QueryRowContext(ctx, `SELECT id, name, desc, id_list, id_board FROM cards WHERE id = ?`, cardID).
		Scan(&card.ID, &card.Name, &card.Desc, &card.IDList, &card.IDBoard)
	if errors.Is(err, sql.ErrNoRows) {
		return trelloclient.Card{}, fmt.Errorf("localboard: card not found: %s", cardID)
	}
	if err != nil {
		return trelloclient.Card{}, fmt.Errorf("localboard: get card: %w", err)
	}
	card.FirstLine = firstNonEmptyLine(card.Desc)
	return card, nil
}

func (s *Store) ListCards(ctx context.Context) ([]CardView, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, desc, id_list, id_board FROM cards ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("localboard: list cards: %w", err)
	}
	out := make([]CardView, 0)
	for rows.Next() {
		var card trelloclient.Card
		if err := rows.Scan(&card.ID, &card.Name, &card.Desc, &card.IDList, &card.IDBoard); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("localboard: scan card: %w", err)
		}
		card.FirstLine = firstNonEmptyLine(card.Desc)
		out = append(out, CardView{Card: card, Comments: []trelloclient.Comment{}})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("localboard: read cards: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("localboard: close cards rows: %w", err)
	}
	for i := range out {
		comments, err := s.ListCommentsSince(ctx, out[i].ID, time.Time{})
		if err != nil {
			return nil, err
		}
		out[i].Comments = comments
	}
	return out, nil
}

func (s *Store) GetCardList(ctx context.Context, cardID string) (trelloclient.List, error) {
	card, err := s.GetCard(ctx, cardID)
	if err != nil {
		return trelloclient.List{}, err
	}
	column, ok := ColumnByID[card.IDList]
	if !ok {
		return trelloclient.List{}, fmt.Errorf("localboard: card %s has unknown list %q", cardID, card.IDList)
	}
	return trelloclient.List{ID: column.ID, Name: column.Name}, nil
}

func (s *Store) ListBoardLists(ctx context.Context, boardID string) ([]trelloclient.List, error) {
	if strings.TrimSpace(boardID) == "" {
		return nil, errors.New("localboard: board id is empty")
	}
	out := make([]trelloclient.List, 0, len(Columns))
	for _, column := range Columns {
		out = append(out, trelloclient.List{ID: column.ID, Name: column.Name})
	}
	return out, nil
}

func (s *Store) MoveCard(ctx context.Context, cardID, targetListID string) error {
	if strings.TrimSpace(cardID) == "" {
		return errors.New("localboard: card id is empty")
	}
	if _, ok := ColumnByID[targetListID]; !ok {
		return fmt.Errorf("localboard: unknown target list id %q", targetListID)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE cards SET id_list = ?, updated_at = ? WHERE id = ?`, targetListID, s.now().UTC().Format(time.RFC3339Nano), cardID)
	if err != nil {
		return fmt.Errorf("localboard: move card: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("localboard: card not found: %s", cardID)
	}
	s.signalChange()
	return nil
}

func (s *Store) AddHumanComment(ctx context.Context, cardID, body string) (trelloclient.Comment, error) {
	return s.addComment(ctx, cardID, body, HumanName)
}

func (s *Store) AddComment(ctx context.Context, cardID, body string) (trelloclient.Comment, error) {
	return s.addComment(ctx, cardID, body, AgentName)
}

func (s *Store) addComment(ctx context.Context, cardID, body, by string) (trelloclient.Comment, error) {
	if strings.TrimSpace(cardID) == "" {
		return trelloclient.Comment{}, errors.New("localboard: card id is empty")
	}
	text := strings.TrimSpace(body)
	if text == "" {
		return trelloclient.Comment{}, errors.New("localboard: comment body is empty")
	}
	if _, err := s.GetCard(ctx, cardID); err != nil {
		return trelloclient.Comment{}, err
	}
	comment := trelloclient.Comment{
		ID:   newID("cm"),
		Text: text,
		By:   by,
		ByID: strings.ToLower(strings.ReplaceAll(by, " ", "-")),
		At:   s.now().UTC(),
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO comments (id, card_id, text, author, at) VALUES (?, ?, ?, ?, ?)`,
		comment.ID, cardID, comment.Text, comment.By, comment.At.Format(time.RFC3339Nano))
	if err != nil {
		return trelloclient.Comment{}, fmt.Errorf("localboard: insert comment: %w", err)
	}
	s.signalChange()
	return comment, nil
}

func (s *Store) GetLatestComment(ctx context.Context, cardID string) (trelloclient.Comment, error) {
	rows, err := s.queryComments(ctx, cardID, `ORDER BY at DESC LIMIT 1`, nil)
	if err != nil {
		return trelloclient.Comment{}, err
	}
	if len(rows) == 0 {
		return trelloclient.Comment{}, fmt.Errorf("localboard: card %s: %w", cardID, trelloclient.ErrNoComments)
	}
	return rows[0], nil
}

func (s *Store) ListCommentsSince(ctx context.Context, cardID string, since time.Time) ([]trelloclient.Comment, error) {
	if since.IsZero() {
		return s.queryComments(ctx, cardID, `ORDER BY at ASC`, nil)
	}
	return s.queryComments(ctx, cardID, `AND at > ? ORDER BY at ASC`, []any{since.UTC().Format(time.RFC3339Nano)})
}

func (s *Store) queryComments(ctx context.Context, cardID, suffix string, args []any) ([]trelloclient.Comment, error) {
	if strings.TrimSpace(cardID) == "" {
		return nil, errors.New("localboard: card id is empty")
	}
	queryArgs := append([]any{cardID}, args...)
	rows, err := s.db.QueryContext(ctx, `SELECT id, text, author, at FROM comments WHERE card_id = ? `+suffix, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("localboard: query comments: %w", err)
	}
	out := make([]trelloclient.Comment, 0)
	for rows.Next() {
		var comment trelloclient.Comment
		var at string
		if err := rows.Scan(&comment.ID, &comment.Text, &comment.By, &at); err != nil {
			return nil, fmt.Errorf("localboard: scan comment: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return nil, fmt.Errorf("localboard: parse comment time: %w", err)
		}
		comment.At = parsed
		comment.ByID = strings.ToLower(strings.ReplaceAll(comment.By, " ", "-"))
		out = append(out, comment)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("localboard: read comments: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("localboard: close comment rows: %w", err)
	}
	return out, nil
}

func (s *Store) ListTokenWebhooks(context.Context, string) ([]trelloclient.Webhook, error) {
	return nil, errors.New("localboard: webhooks are not supported")
}

func (s *Store) UpdateWebhookCallback(context.Context, string, string) error {
	return errors.New("localboard: webhooks are not supported")
}

func (s *Store) CreateTokenWebhook(context.Context, string, string, string, string) (trelloclient.Webhook, error) {
	return trelloclient.Webhook{}, errors.New("localboard: webhooks are not supported")
}

func (s *Store) DeleteWebhook(context.Context, string, string) error {
	return errors.New("localboard: webhooks are not supported")
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
