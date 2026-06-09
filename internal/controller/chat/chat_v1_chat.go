package chat

import (
	"SREagent/api/chat/v1"
	"SREagent/internal/ai/agent/chat_pipeline"
	"SREagent/internal/ai/agent/trace"
	"SREagent/internal/authz"
	"SREagent/utility/log_call_back"
	"SREagent/utility/mem"
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	userID, conversationID := resolveMemoryScope(req.Id, req.UserId, req.ConversationId)
	principal := authz.PrincipalFromRequest(ctx, g.RequestFromCtx(ctx), userID)
	ctx = authz.WithPrincipal(ctx, principal)
	ctx = context.WithValue(ctx, "client_id", conversationID)
	if principal.UserID != "" {
		userID = principal.UserID
	}
	msg := req.Question
	memory := mem.GetConversationMemory(userID, conversationID)
	layers, err := mem.LoadContextLayers(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	userMessage := &chat_pipeline.UserMessage{
		ID:             conversationID,
		UserID:         userID,
		ConversationID: conversationID,
		Query:          msg,
		History:        memory.GetContextMessages(mem.DefaultContextPolicy()),
		Summary:        memory.GetSummary(),
		SessionState:   layers.SessionState,
		DurableMemory:  layers.DurableMemory,
		ToolResults:    layers.ToolResults,
	}

	runner, err := chat_pipeline.BuildChatAgent(ctx)
	if err != nil {
		return nil, err
	}

	recorder := trace.NewRecorder()
	out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(
		log_call_back.LogCallback(nil),
		trace.Callback("chat_agent", recorder),
		mem.ToolResultCallback(userID, conversationID),
	))
	if err != nil {
		return nil, err
	}
	answer, err := chat_pipeline.EnforceEvidenceAnswer(ctx, out.Content, recorder.Steps())
	if err != nil {
		return nil, err
	}
	res = &v1.ChatRes{
		Answer: answer,
	}
	memory.SetMessages(schema.UserMessage(msg))
	memory.SetMessages(schema.SystemMessage(answer))
	if err = mem.UpdateSessionState(ctx, userID, conversationID, msg, answer); err != nil {
		return nil, err
	}

	return res, nil
}
