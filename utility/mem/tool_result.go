package mem

import (
	"SREagent/internal/ai/evidence"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"github.com/gogf/gf/v2/frame/g"
)

const (
	toolResultContextLimit    = 5
	toolResultSummaryMaxChars = 1600
)

type ToolResult struct {
	ToolName      string
	InputJSON     string
	OutputSummary string
	OutputJSON    string
	Status        string
}

type toolResultRow struct {
	ToolName      string `json:"tool_name"`
	OutputSummary string `json:"output_summary"`
	Status        string `json:"status"`
}

type toolResultRecorder struct {
	storageID string
	mu        sync.Mutex
	inputs    map[string][]string
}

func ToolResultCallback(userID string, conversationID string) callbacks.Handler {
	scope := normalizeMemoryScope(userID, conversationID)
	recorder := &toolResultRecorder{
		storageID: strings.TrimSpace(scope.StorageID),
		inputs:    make(map[string][]string),
	}
	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(recorder.onStart)
	builder.OnEndFn(recorder.onEnd)
	builder.OnErrorFn(recorder.onError)
	return builder.Build()
}

func SaveToolResult(ctx context.Context, userID string, conversationID string, result ToolResult) error {
	scope := normalizeMemoryScope(userID, conversationID)
	return saveToolResultByStorageID(ctx, scope.StorageID, result)
}

func loadRecentToolResults(ctx context.Context, conversationID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", nil
	}

	var results []toolResultRow
	if err := g.DB().Model("tool_results").
		Ctx(ctx).
		Fields("tool_name", "output_summary", "status").
		Where("conversation_id", conversationID).
		OrderDesc("id").
		Limit(toolResultContextLimit).
		Scan(&results); err != nil {
		return "", err
	}
	return formatToolResults(results), nil
}

func formatToolResults(results []toolResultRow) string {
	lines := make([]string, 0, len(results))
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		name := strings.TrimSpace(result.ToolName)
		summary := strings.TrimSpace(result.OutputSummary)
		if name == "" || summary == "" {
			continue
		}
		status := strings.TrimSpace(result.Status)
		if status == "" {
			status = "success"
		}
		lines = append(lines, fmt.Sprintf("- %s [%s]: %s", name, status, summary))
	}
	return strings.Join(lines, "\n")
}

func (r *toolResultRecorder) onStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if r == nil || info == nil || info.Component != components.ComponentOfTool {
		return ctx
	}
	toolInput := tool.ConvCallbackInput(input)
	if toolInput == nil {
		return ctx
	}
	r.pushInput(info.Name, toolInput.ArgumentsInJSON)
	return ctx
}

func (r *toolResultRecorder) onEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if r == nil || info == nil || info.Component != components.ComponentOfTool {
		return ctx
	}
	toolOutput := tool.ConvCallbackOutput(output)
	response := ""
	if toolOutput != nil {
		response = toolOutput.Response
	}
	parsed := evidence.Parse(response).Envelope
	_ = saveToolResultByStorageID(ctx, r.storageID, ToolResult{
		ToolName:      info.Name,
		InputJSON:     r.popInput(info.Name),
		OutputSummary: formatEnvelopeSummary(parsed, response),
		OutputJSON:    response,
		Status:        parsed.Status,
	})
	return ctx
}

func (r *toolResultRecorder) onError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if r == nil || info == nil || info.Component != components.ComponentOfTool {
		return ctx
	}
	_ = saveToolResultByStorageID(ctx, r.storageID, ToolResult{
		ToolName:      info.Name,
		InputJSON:     r.popInput(info.Name),
		OutputSummary: formatEnvelopeSummary(evidence.NewError(err, "tool callback error"), ""),
		Status:        evidence.StatusError,
	})
	return ctx
}

func formatEnvelopeSummary(env evidence.Envelope, raw string) string {
	summary := strings.TrimSpace(env.Summary)
	if summary == "" {
		summary = strings.TrimSpace(env.Message)
	}
	if summary == "" {
		summary = strings.TrimSpace(env.Error)
	}
	if summary == "" {
		summary = strings.TrimSpace(raw)
	}
	if env.CanUseAsEvidence && strings.TrimSpace(env.EvidenceID) != "" {
		return fmt.Sprintf("%s evidence_id=%s data_status=%s", summary, env.EvidenceID, env.DataStatus)
	}
	if strings.TrimSpace(env.FailedQueryID) != "" {
		return fmt.Sprintf("%s failed_query_id=%s data_status=%s", summary, env.FailedQueryID, env.DataStatus)
	}
	return summary
}

func saveToolResultByStorageID(ctx context.Context, storageID string, result ToolResult) error {
	storageID = strings.TrimSpace(storageID)
	result.ToolName = strings.TrimSpace(result.ToolName)
	if storageID == "" || result.ToolName == "" {
		return nil
	}
	if strings.TrimSpace(result.Status) == "" {
		result.Status = "success"
	}

	data := g.Map{
		"conversation_id": storageID,
		"tool_name":       result.ToolName,
		"output_summary":  trimSessionStateText(result.OutputSummary, toolResultSummaryMaxChars),
		"status":          result.Status,
	}
	if json.Valid([]byte(result.InputJSON)) {
		data["input"] = result.InputJSON
	}
	if json.Valid([]byte(result.OutputJSON)) {
		data["output"] = result.OutputJSON
	}
	_, err := g.DB().Model("tool_results").Ctx(ctx).Data(data).Insert()
	return err
}

func (r *toolResultRecorder) pushInput(name string, input string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inputs[name] = append(r.inputs[name], input)
}

func (r *toolResultRecorder) popInput(name string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := r.inputs[name]
	if len(values) == 0 {
		return ""
	}
	value := values[0]
	r.inputs[name] = values[1:]
	return value
}
