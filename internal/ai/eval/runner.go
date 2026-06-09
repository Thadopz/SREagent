package eval

import (
	"SREagent/internal/ai/agent/chat_pipeline"
	"SREagent/internal/ai/agent/trace"
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func RunCases(ctx context.Context, cases []Case, opts RunOptions) ([]EvalCaseResult, error) {
	if opts.Mode == "" {
		opts.Mode = ModeMock
	}
	if opts.RunID == "" {
		opts.RunID = time.Now().Format("20060102-150405")
	}

	results := make([]EvalCaseResult, 0, len(cases))
	for _, c := range cases {
		caseCtx := ctx
		cancel := func() {}
		if opts.CaseTimeoutSeconds > 0 {
			caseCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.CaseTimeoutSeconds)*time.Second)
		}
		result, err := RunCase(caseCtx, c, opts)
		cancel()
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func RunCase(ctx context.Context, c Case, opts RunOptions) (EvalCaseResult, error) {
	started := time.Now()
	conversationID := fmt.Sprintf("eval-%s-%s", sanitizeID(c.ID), opts.RunID)
	userMessage := &chat_pipeline.UserMessage{
		ID:             conversationID,
		UserID:         "eval",
		ConversationID: conversationID,
		Query:          c.Query,
		History:        schemaHistory(c.History),
		Summary:        c.MockContext.Summary,
		SessionState:   c.MockContext.SessionState,
		DurableMemory:  c.MockContext.DurableMemory,
		ToolResults:    c.MockContext.ToolResults,
		KnowledgeMode:  chat_pipeline.KnowledgeAuto,
	}

	modelContext := chat_pipeline.AssembleModelContext(ctx, userMessage, started)
	result := EvalCaseResult{
		ID:             c.ID,
		Category:       c.Category,
		Mode:           opts.Mode,
		Query:          c.Query,
		ConversationID: conversationID,
		StartedAt:      started,
		Context: ContextEvidence{
			SummaryChars:       utf8.RuneCountInString(modelContext.Summary),
			SessionStateChars:  utf8.RuneCountInString(modelContext.SessionState),
			DurableMemoryChars: utf8.RuneCountInString(modelContext.DurableMemory),
			ToolResultsChars:   utf8.RuneCountInString(modelContext.ToolResults),
			HistoryMessages:    len(modelContext.History),
			UseKnowledge:       modelContext.UseKnowledge,
			PromptContextChars: promptContextChars(modelContext),
		},
	}

	recorder := trace.NewRecorder()
	var retrieved []DocEvidence
	agentOpts := chat_pipeline.ChatAgentOptions{
		OnRetrieval: func(obs chat_pipeline.RetrievalObservation) {
			result.Context.UseKnowledge = obs.Triggered
			for _, doc := range obs.Docs {
				retrieved = append(retrieved, docFromPipelineDoc(doc))
			}
		},
	}
	if opts.Mode == ModeMock {
		agentOpts.ChatModel = &fakeModel{caseData: c, toolNames: c.Expected.Tools}
		agentOpts.Retriever = fakeRetriever{docs: c.MockContext.Documents}
		agentOpts.Tools = fakeTools(c)
		agentOpts.SkipRerank = true
		agentOpts.ReactMaxStep = 8
	}

	runner, err := chat_pipeline.BuildChatAgentWithOptions(ctx, agentOpts)
	if err != nil {
		result.Error = err.Error()
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Scores = ScoreCase(c, result)
		return result, nil
	}

	out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(trace.Callback("eval_chat_agent", recorder)))
	if err != nil {
		result.Error = err.Error()
		if ctx.Err() != nil {
			result.Error = ctx.Err().Error()
		}
	} else if out != nil {
		result.Answer = out.Content
	}
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.TraceSteps = recorder.Steps()
	result.RetrievedDocs = retrieved
	result.ToolCalls = toolEvidenceFromTrace(result.TraceSteps)
	result.Scores = ScoreCase(c, result)
	return result, nil
}

func schemaHistory(history []ChatMessage) []*schema.Message {
	messages := make([]*schema.Message, 0, len(history))
	for _, msg := range history {
		switch strings.ToLower(strings.TrimSpace(msg.Role)) {
		case "assistant":
			messages = append(messages, schema.AssistantMessage(msg.Content, nil))
		case "system":
			messages = append(messages, schema.SystemMessage(msg.Content))
		default:
			messages = append(messages, schema.UserMessage(msg.Content))
		}
	}
	return messages
}

func promptContextChars(c chat_pipeline.ModelContext) int {
	total := utf8.RuneCountInString(c.Content) +
		utf8.RuneCountInString(c.Summary) +
		utf8.RuneCountInString(c.SessionState) +
		utf8.RuneCountInString(c.DurableMemory) +
		utf8.RuneCountInString(c.ToolResults) +
		utf8.RuneCountInString(c.RequestAnalysis)
	for _, msg := range c.History {
		if msg != nil {
			total += utf8.RuneCountInString(msg.Content)
		}
	}
	return total
}

func toolEvidenceFromTrace(steps []trace.AgentStep) []ToolEvidence {
	starts := map[string]trace.AgentStep{}
	var tools []ToolEvidence
	for _, step := range steps {
		if step.Component != "Tool" && step.Component != "tool" {
			continue
		}
		key := step.Name
		if key == "" {
			key = fmt.Sprintf("%d", step.Index)
		}
		switch step.Phase {
		case "start":
			starts[key] = step
		case "end":
			start := starts[key]
			tools = append(tools, ToolEvidence{
				Name:    step.Name,
				Input:   start.Input,
				Output:  step.Output,
				Status:  "success",
				Elapsed: step.ElapsedMS,
			})
		case "error":
			start := starts[key]
			tools = append(tools, ToolEvidence{
				Name:   step.Name,
				Input:  start.Input,
				Status: "error",
				Error:  step.Error,
			})
		}
	}
	return tools
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "case"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, s)
}
