package localboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lonegunmanb/jjc/internal/app/trelloclient"
)

func BuildCreatePayload(card trelloclient.Card, list Column) ([]byte, error) {
	if err := validateCardAndColumn(card, list); err != nil {
		return nil, err
	}
	return marshalAction("createCard", map[string]any{
		"card":  cardObject(card),
		"list":  listObject(list),
		"board": boardObject(),
	})
}

func BuildMovePayload(card trelloclient.Card, from, to Column) ([]byte, error) {
	if err := validateCardAndColumn(card, to); err != nil {
		return nil, err
	}
	if strings.TrimSpace(from.ID) == "" || strings.TrimSpace(from.Name) == "" {
		return nil, errors.New("localboard: source column is incomplete")
	}
	return marshalAction("updateCard", map[string]any{
		"card":       cardObject(card),
		"listBefore": listObject(from),
		"listAfter":  listObject(to),
		"board":      boardObject(),
	})
}

func BuildCommentPayload(card trelloclient.Card, comment trelloclient.Comment) ([]byte, error) {
	if strings.TrimSpace(card.ID) == "" {
		return nil, errors.New("localboard: card id is empty")
	}
	if strings.TrimSpace(comment.Text) == "" {
		return nil, errors.New("localboard: comment text is empty")
	}
	memberName := comment.By
	if strings.TrimSpace(memberName) == "" {
		memberName = HumanName
	}
	return marshalActionWithMember("commentCard", map[string]any{
		"card":  cardObject(card),
		"text":  comment.Text,
		"board": boardObject(),
	}, memberName)
}

func validateCardAndColumn(card trelloclient.Card, column Column) error {
	if strings.TrimSpace(card.ID) == "" {
		return errors.New("localboard: card id is empty")
	}
	if strings.TrimSpace(column.ID) == "" || strings.TrimSpace(column.Name) == "" {
		return errors.New("localboard: column is incomplete")
	}
	return nil
}

func marshalAction(actionType string, data map[string]any) ([]byte, error) {
	return marshalActionWithMember(actionType, data, HumanName)
}

func marshalActionWithMember(actionType string, data map[string]any, memberName string) ([]byte, error) {
	payload := map[string]any{
		"action": map[string]any{
			"id":   newID("local_action"),
			"type": actionType,
			"date": time.Now().UTC().Format(time.RFC3339Nano),
			"data": data,
			"memberCreator": map[string]any{
				"id":       "local-operator",
				"fullName": memberName,
			},
		},
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("localboard: marshal payload: %w", err)
	}
	return out, nil
}

func cardObject(card trelloclient.Card) map[string]any {
	return map[string]any{
		"id":      card.ID,
		"name":    card.Name,
		"desc":    card.Desc,
		"idList":  card.IDList,
		"idBoard": fallback(card.IDBoard, DefaultBoardID),
	}
}

func listObject(column Column) map[string]any {
	return map[string]any{"id": column.ID, "name": column.Name}
}

func boardObject() map[string]any {
	return map[string]any{"id": DefaultBoardID, "name": DefaultBoardName}
}

func fallback(v, fb string) string {
	if v == "" {
		return fb
	}
	return v
}
