package mem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	defaultSessionStatus        = "active"
	sessionStateGoalMaxChars    = 240
	sessionStateMessageMaxChars = 1200
)

type sessionStatePayload struct {
	RecentUserQuery     string `json:"recent_user_query"`
	RecentAssistantTurn string `json:"recent_assistant_turn"`
}

func UpdateSessionState(ctx context.Context, userID string, conversationID string, userQuery string, assistantTurn string) error {
	scope := normalizeMemoryScope(userID, conversationID)
	userQuery = strings.TrimSpace(userQuery)
	assistantTurn = strings.TrimSpace(assistantTurn)
	if scope.StorageID == "" || (userQuery == "" && assistantTurn == "") {
		return nil
	}

	existing, err := loadSessionStateRow(ctx, scope.StorageID)
	if err != nil {
		return err
	}

	goal := strings.TrimSpace(existing.Goal)
	if goal == "" {
		goal = trimSessionStateText(userQuery, sessionStateGoalMaxChars)
	}

	payload, err := json.Marshal(sessionStatePayload{
		RecentUserQuery:     trimSessionStateText(userQuery, sessionStateMessageMaxChars),
		RecentAssistantTurn: trimSessionStateText(assistantTurn, sessionStateMessageMaxChars),
	})
	if err != nil {
		return err
	}

	if _, err = g.DB().Model("session_states").
		Ctx(ctx).
		Data(g.Map{
			"conversation_id": scope.StorageID,
			"goal":            goal,
			"status":          defaultSessionStatus,
			"state":           string(payload),
		}).
		InsertIgnore(); err != nil {
		return err
	}

	_, err = g.DB().Model("session_states").
		Ctx(ctx).
		Where("conversation_id", scope.StorageID).
		Data(g.Map{
			"goal":   goal,
			"status": defaultSessionStatus,
			"state":  string(payload),
		}).
		Update()
	return err
}

func loadSessionStateRow(ctx context.Context, conversationID string) (sessionStateRow, error) {
	var state sessionStateRow
	err := g.DB().Model("session_states").
		Ctx(ctx).
		Fields("goal", "status", "state").
		Where("conversation_id", conversationID).
		Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	return state, err
}

func trimSessionStateText(value string, maxChars int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if maxChars <= 0 || utf8.RuneCountInString(value) <= maxChars {
		return value
	}
	return string([]rune(value)[:maxChars])
}
