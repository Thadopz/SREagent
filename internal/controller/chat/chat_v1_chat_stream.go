package chat

import (
	v1 "SREagent/api/chat/v1"
	"SREagent/internal/ai/agent/chat_pipeline"
	"SREagent/internal/ai/agent/trace"
	"SREagent/internal/authz"
	"SREagent/utility/log_call_back"
	"SREagent/utility/mem"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) ChatStream(ctx context.Context, req *v1.ChatStreamReq) (res *v1.ChatStreamRes, err error) {
	userID, conversationID := resolveMemoryScope(req.Id, req.UserId, req.ConversationId)
	principal := authz.PrincipalFromRequest(ctx, g.RequestFromCtx(ctx), userID)
	ctx = authz.WithPrincipal(ctx, principal)
	if principal.UserID != "" {
		userID = principal.UserID
	}
	msg := req.Question

	ctx = context.WithValue(ctx, "client_id", conversationID)
	client, err := c.service.Create(ctx, g.RequestFromCtx(ctx))
	if err != nil {
		return nil, err
	}

	memory := mem.GetConversationMemory(userID, conversationID)
	layers, err := mem.LoadContextLayers(ctx, userID, conversationID)
	if err != nil {
		client.SendToClient("error", err.Error())
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
		client.SendToClient("error", err.Error())
		return nil, err
	}
	recorder := trace.NewRecorder()
	client.SendToClient("thinking", `{"message":"analyzing request"}`)
	sr, err := runner.Stream(ctx, userMessage, compose.WithCallbacks(
		log_call_back.LogCallback(nil),
		trace.StreamingCallback("chat_stream_agent", recorder, func(event string, step trace.AgentStep) {
			client.SendToClient(event, marshalTraceStep(step))
		}),
		mem.ToolResultCallback(userID, conversationID),
	))
	if err != nil {
		client.SendToClient("error", err.Error())
		return nil, err
	}
	defer sr.Close()

	var fullResponse strings.Builder

	defer func() {
		completeResponse := fullResponse.String()
		if completeResponse != "" {
			memory.SetMessages(schema.UserMessage(msg))
			memory.SetMessages(schema.SystemMessage(completeResponse))
			if stateErr := mem.UpdateSessionState(ctx, userID, conversationID, msg, completeResponse); stateErr != nil {
				client.SendToClient("error", stateErr.Error())
			}
		}
	}()

	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			originalResponse := fullResponse.String()
			guarded, guardErr := chat_pipeline.EnforceEvidenceAnswer(ctx, originalResponse, recorder.Steps())
			if guardErr != nil {
				client.SendToClient("error", guardErr.Error())
				return &v1.ChatStreamRes{}, nil
			}
			if guarded != originalResponse {
				fullResponse.Reset()
				fullResponse.WriteString(guarded)
				client.SendToClient("replace_message", marshalStreamContent(guarded))
			}
			client.SendToClient("done", "Stream completed")
			return &v1.ChatStreamRes{}, nil
		}
		if err != nil {
			client.SendToClient("error", err.Error())
			return &v1.ChatStreamRes{}, nil
		}
		fullResponse.WriteString(chunk.Content)
		if chunk.Content != "" {
			client.SendToClient("message", chunk.Content)
		}
	}
}

func marshalTraceStep(step trace.AgentStep) string {
	b, err := json.Marshal(step)
	if err != nil {
		return `{"error":"marshal trace step failed"}`
	}
	return string(b)
}

func marshalStreamContent(content string) string {
	b, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return `{"content":""}`
	}
	return string(b)
}
