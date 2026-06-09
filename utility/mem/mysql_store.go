package mem

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

type mysqlMemoryStore struct{}

type mysqlTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func newMySQLMemoryStore() *mysqlMemoryStore {
	return &mysqlMemoryStore{}
}

func (s *mysqlMemoryStore) Load(id string) (*memorySnapshot, error) {
	ctx := context.Background()

	var turns []mysqlTurn
	if err := g.DB().Model("conversation_turns").
		Ctx(ctx).
		Fields("role", "content").
		Where("conversation_id", id).
		OrderDesc("id").
		Limit(defaultStoredWindowSize).
		Scan(&turns); err != nil {
		return nil, err
	}

	summaryRecord, err := g.DB().Model("conversation_summaries").
		Ctx(ctx).
		Fields("summary", "updated_at").
		Where("conversation_id", id).
		One()
	if err != nil {
		return nil, err
	}
	if len(turns) == 0 && summaryRecord.IsEmpty() {
		return nil, nil
	}

	snapshot := &memorySnapshot{
		ID:            id,
		Messages:      make([]*schema.Message, 0, len(turns)),
		MaxWindowSize: defaultStoredWindowSize,
		UpdatedAt:     time.Now(),
	}
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		snapshot.Messages = append(snapshot.Messages, &schema.Message{
			Role:    schema.RoleType(turn.Role),
			Content: turn.Content,
		})
	}
	if !summaryRecord.IsEmpty() {
		snapshot.Summary = summaryRecord["summary"].String()
		if updatedAt := summaryRecord["updated_at"].Time(); !updatedAt.IsZero() {
			snapshot.UpdatedAt = updatedAt
		}
	}
	return snapshot, nil
}

func (s *mysqlMemoryStore) Save(snapshot *memorySnapshot) error {
	if snapshot == nil || strings.TrimSpace(snapshot.ID) == "" {
		return nil
	}

	return g.DB().Transaction(context.Background(), func(ctx context.Context, tx gdb.TX) error {
		if err := ensureConversation(ctx, tx, snapshot); err != nil {
			return err
		}

		if _, err := tx.Model("conversation_turns").
			Ctx(ctx).
			Where("conversation_id", snapshot.ID).
			Delete(); err != nil {
			return err
		}

		if len(snapshot.Messages) > 0 {
			turns := make(g.List, 0, len(snapshot.Messages))
			for _, msg := range snapshot.Messages {
				if msg == nil {
					continue
				}
				turns = append(turns, g.Map{
					"conversation_id": snapshot.ID,
					"role":            string(msg.Role),
					"content":         msg.Content,
				})
			}
			if len(turns) > 0 {
				if _, err := tx.Model("conversation_turns").Ctx(ctx).Data(turns).Insert(); err != nil {
					return err
				}
			}
		}

		if strings.TrimSpace(snapshot.Summary) == "" {
			_, err := tx.Model("conversation_summaries").
				Ctx(ctx).
				Where("conversation_id", snapshot.ID).
				Delete()
			return err
		}

		return upsertSummary(ctx, tx, snapshot)
	})
}

func (s *mysqlMemoryStore) Append(snapshot *memorySnapshot, msg *schema.Message) error {
	if snapshot == nil || msg == nil || strings.TrimSpace(snapshot.ID) == "" {
		return nil
	}

	return g.DB().Transaction(context.Background(), func(ctx context.Context, tx gdb.TX) error {
		if err := ensureConversation(ctx, tx, snapshot); err != nil {
			return err
		}

		if _, err := tx.Model("conversation_turns").
			Ctx(ctx).
			Data(g.Map{
				"conversation_id": snapshot.ID,
				"role":            string(msg.Role),
				"content":         msg.Content,
			}).
			Insert(); err != nil {
			return err
		}
		return upsertSummary(ctx, tx, snapshot)
	})
}

func ensureConversation(ctx context.Context, tx gdb.TX, snapshot *memorySnapshot) error {
	if _, err := tx.Model("conversations").
		Ctx(ctx).
		Data(g.Map{
			"conversation_id": snapshot.ID,
			"user_id":         normalizeScopeID(snapshot.UserID, snapshot.ID),
			"updated_at":      snapshot.UpdatedAt,
		}).
		InsertIgnore(); err != nil {
		return err
	}

	_, err := tx.Model("conversations").
		Ctx(ctx).
		Where("conversation_id", snapshot.ID).
		Data(g.Map{"updated_at": snapshot.UpdatedAt}).
		Update()
	return err
}

func upsertSummary(ctx context.Context, tx gdb.TX, snapshot *memorySnapshot) error {
	if strings.TrimSpace(snapshot.Summary) == "" {
		return nil
	}

	if _, err := tx.Model("conversation_summaries").
		Ctx(ctx).
		Data(g.Map{
			"conversation_id": snapshot.ID,
			"summary":         snapshot.Summary,
			"updated_at":      snapshot.UpdatedAt,
		}).
		InsertIgnore(); err != nil {
		return err
	}

	_, err := tx.Model("conversation_summaries").
		Ctx(ctx).
		Where("conversation_id", snapshot.ID).
		Data(g.Map{
			"summary":    snapshot.Summary,
			"updated_at": snapshot.UpdatedAt,
		}).
		Update()
	return err
}
