package chat_pipeline

import (
	"SREagent/internal/ai/input_preprocessor"
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

type KnowledgeMode string

const (
	KnowledgeAuto   KnowledgeMode = "auto"
	KnowledgeAlways KnowledgeMode = "always"
	KnowledgeNever  KnowledgeMode = "never"
)

type ModelContext struct {
	Content         string
	History         []*schema.Message
	Summary         string
	SessionState    string
	DurableMemory   string
	ToolResults     string
	ProjectContext  string
	RequestAnalysis string
	Skills          string
	Date            string
	KnowledgeQuery  string
	UseKnowledge    bool
}

func AssembleModelContext(ctx context.Context, input *UserMessage, now time.Time) ModelContext {
	if input == nil {
		return ModelContext{Date: now.Format("2006-01-02 15:04:05")}
	}

	processed := input_preprocessor.Process(ctx, input.Query, now)
	return ModelContext{
		Content:         processed.CleanQuery,
		History:         budgetHistoryMessages(input.History, HistoryMaxChars),
		Summary:         trimRunesHead(strings.TrimSpace(input.Summary), SummaryMaxChars),
		SessionState:    trimRunesHead(strings.TrimSpace(input.SessionState), SessionStateMaxChars),
		DurableMemory:   trimRunesHead(strings.TrimSpace(input.DurableMemory), DurableMemoryMaxChars),
		ToolResults:     trimRunesHead(strings.TrimSpace(input.ToolResults), ToolResultsMaxChars),
		RequestAnalysis: processed.FormatPrompt(),
		Date:            now.Format("2006-01-02 15:04:05"),
		KnowledgeQuery:  processed.KnowledgeQuery,
		UseKnowledge:    shouldRetrieveKnowledge(processed.CleanQuery, input.KnowledgeMode, processed.NeedKnowledge),
	}
}

func (c ModelContext) PromptValues() map[string]any {
	return map[string]any{
		"content":          c.Content,
		"history":          c.History,
		"summary":          c.Summary,
		"session_state":    c.SessionState,
		"durable_memory":   c.DurableMemory,
		"tool_results":     c.ToolResults,
		"project_context":  c.ProjectContext,
		"request_analysis": c.RequestAnalysis,
		"skills":           c.Skills,
		"date":             c.Date,
	}
}

func shouldRetrieveKnowledge(query string, mode KnowledgeMode, preprocessedNeed bool) bool {
	query = strings.TrimSpace(query)
	switch mode {
	case KnowledgeAlways:
		return query != ""
	case KnowledgeNever:
		return false
	default:
		return preprocessedNeed || shouldAutoRetrieveKnowledge(query)
	}
}

func shouldAutoRetrieveKnowledge(query string) bool {
	if query == "" {
		return false
	}

	phrases := loadKnowledgeRetrievalPhrases(context.Background())
	normalized := strings.ToLower(query)
	for _, phrase := range phrases.Off {
		if strings.Contains(normalized, phrase) {
			return false
		}
	}
	for _, phrase := range phrases.On {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	for _, keyword := range phrases.Keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

type knowledgeRetrievalPhrases struct {
	On       []string
	Off      []string
	Keywords []string
}

func loadKnowledgeRetrievalPhrases(ctx context.Context) knowledgeRetrievalPhrases {
	return knowledgeRetrievalPhrases{
		On:       mergeNormalizedPhrases(defaultExplicitKnowledgeOnPhrases, cfgStringSlice(ctx, "knowledge_retrieval.force_phrases")),
		Off:      mergeNormalizedPhrases(defaultExplicitKnowledgeOffPhrases, cfgStringSlice(ctx, "knowledge_retrieval.suppress_phrases")),
		Keywords: mergeNormalizedPhrases(defaultKnowledgeIntentKeywords, cfgStringSlice(ctx, "knowledge_retrieval.auto_keywords")),
	}
}

func cfgStringSlice(ctx context.Context, key string) []string {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil || v == nil {
		return nil
	}
	return v.Strings()
}

func mergeNormalizedPhrases(base []string, extra []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	merged := make([]string, 0, len(base)+len(extra))
	for _, phrase := range append(base, extra...) {
		phrase = strings.ToLower(strings.TrimSpace(phrase))
		if phrase == "" {
			continue
		}
		if _, ok := seen[phrase]; ok {
			continue
		}
		seen[phrase] = struct{}{}
		merged = append(merged, phrase)
	}
	return merged
}

var defaultExplicitKnowledgeOffPhrases = []string{
	"\u4e0d\u67e5\u77e5\u8bc6\u5e93",
	"\u4e0d\u7528\u77e5\u8bc6\u5e93",
	"\u4e0d\u67e5\u6587\u6863",
	"\u4e0d\u7528\u6587\u6863",
	"\u53ea\u6839\u636e\u5f53\u524d\u5bf9\u8bdd",
	"\u53ea\u57fa\u4e8e\u5f53\u524d\u5bf9\u8bdd",
	"do not search docs",
	"don't search docs",
	"without docs",
}

var defaultExplicitKnowledgeOnPhrases = []string{
	"\u67e5\u6587\u6863",
	"\u67e5\u77e5\u8bc6\u5e93",
	"\u67e5\u8be2\u77e5\u8bc6\u5e93",
	"\u6309\u624b\u518c",
	"\u6839\u636e\u624b\u518c",
	"runbook",
}

var defaultKnowledgeIntentKeywords = []string{
	"\u544a\u8b66",
	"\u62a5\u8b66",
	"\u6545\u969c",
	"\u5f02\u5e38",
	"\u62a5\u9519",
	"\u9519\u8bef",
	"\u65e5\u5fd7",
	"\u6392\u67e5",
	"\u5904\u7406",
	"\u624b\u518c",
	"\u6587\u6863",
	"\u6d41\u7a0b",
	"sop",
	"\u6307\u6807",
	"prometheus",
	"\u8d85\u65f6",
	"\u5931\u8d25",
	"\u6062\u590d",
	"\u539f\u56e0",
	"\u600e\u4e48\u5904\u7406",
	"\u5982\u4f55\u5904\u7406",
	"alert",
	"error",
	"exception",
	"log",
	"troubleshoot",
	"runbook",
	"incident",
	"metric",
	"prometheus",
	"timeout",
	"failure",
}

func cloneHistory(history []*schema.Message) []*schema.Message {
	cloned := make([]*schema.Message, 0, len(history))
	for _, msg := range history {
		if msg == nil {
			continue
		}
		msgCopy := *msg
		cloned = append(cloned, &msgCopy)
	}
	return cloned
}
