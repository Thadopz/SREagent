package chat_pipeline

import "github.com/cloudwego/eino/schema"

type UserMessage struct {
	ID             string            `json:"id"`
	UserID         string            `json:"user_id"`
	ConversationID string            `json:"conversation_id"`
	Query          string            `json:"query"`
	History        []*schema.Message `json:"history"`
	Summary        string            `json:"summary"`
	SessionState   string            `json:"session_state"`
	DurableMemory  string            `json:"durable_memory"`
	ToolResults    string            `json:"tool_results"`
	KnowledgeMode  KnowledgeMode     `json:"knowledge_mode"`
}
