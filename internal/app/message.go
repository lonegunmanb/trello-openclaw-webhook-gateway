package app

import (
	"encoding/json"
	"fmt"
)

func BuildMessage(raw []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Sprintf("Trello: invalid payload %q", string(raw))
	}

	action := asMap(payload["action"])
	actionType := asString(action["type"])
	data := asMap(action["data"])
	card := asMap(data["card"])
	board := asMap(data["board"])
	member := asMap(action["memberCreator"])

	cardName := fallback(asString(card["name"]), "unknown-card")
	boardName := fallback(asString(board["name"]), "unknown-board")
	fullName := fallback(asString(member["fullName"]), "unknown-user")

	listBefore := asMap(data["listBefore"])
	listAfter := asMap(data["listAfter"])
	listBeforeName := asString(listBefore["name"])
	listAfterName := asString(listAfter["name"])

	if listBeforeName != "" && listAfterName != "" {
		return fmt.Sprintf("Trello: 卡片 %q 从 %q 移到 %q (by %s)", cardName, listBeforeName, listAfterName, fullName)
	}

	if actionType == "commentCard" {
		text := fallback(asString(data["text"]), "")
		return fmt.Sprintf("Trello: %s 在卡片 %q 上评论: %s", fullName, cardName, text)
	}

	return fmt.Sprintf("Trello: %s on card %q in board %q by %s raw=%s", fallback(actionType, "unknown-action"), cardName, boardName, fullName, compactJSON(raw))
}

func compactJSON(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func fallback(v, fb string) string {
	if v == "" {
		return fb
	}
	return v
}
