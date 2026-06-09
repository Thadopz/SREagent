package chat_pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

func TestAssembleModelContextBuildsPromptValues(t *testing.T) {
	history := []*schema.Message{schema.UserMessage("old question")}
	input := &UserMessage{
		Query:         "  show the runbook  ",
		History:       history,
		Summary:       "  earlier summary  ",
		SessionState:  "  active incident  ",
		DurableMemory: "  prefer concise answers  ",
		ToolResults:   "  log query returned 3 rows  ",
		KnowledgeMode: KnowledgeAlways,
	}

	got := AssembleModelContext(context.Background(), input, time.Date(2026, time.May, 21, 9, 30, 0, 0, time.UTC))
	if got.Content != "show the runbook" {
		t.Fatalf("expected trimmed content, got %q", got.Content)
	}
	if got.Summary != "earlier summary" {
		t.Fatalf("expected trimmed summary, got %q", got.Summary)
	}
	if got.SessionState != "active incident" || got.DurableMemory != "prefer concise answers" {
		t.Fatalf("expected trimmed memory layers, got state=%q durable=%q", got.SessionState, got.DurableMemory)
	}
	if got.ToolResults != "log query returned 3 rows" {
		t.Fatalf("expected trimmed tool results, got %q", got.ToolResults)
	}
	if got.Date != "2026-05-21 09:30:00" {
		t.Fatalf("expected formatted date, got %q", got.Date)
	}
	if !got.UseKnowledge {
		t.Fatal("expected explicit knowledge mode to use knowledge context")
	}
	if got.KnowledgeQuery != "show the runbook internal documentation runbook" {
		t.Fatalf("expected preprocessed knowledge query, got %q", got.KnowledgeQuery)
	}
	if !strings.Contains(got.RequestAnalysis, "Intent: query") || !strings.Contains(got.RequestAnalysis, "Need Knowledge: true") {
		t.Fatalf("expected request analysis prompt, got %q", got.RequestAnalysis)
	}

	got.ProjectContext = "project rules"
	values := got.PromptValues()
	if values["project_context"] != "project rules" {
		t.Fatalf("expected project context prompt value, got %#v", values["project_context"])
	}
	if values["request_analysis"] != got.RequestAnalysis {
		t.Fatalf("expected request analysis prompt value, got %#v", values["request_analysis"])
	}

	got.History[0].Content = "changed"
	if history[0].Content != "old question" {
		t.Fatalf("expected history clone, got %q", history[0].Content)
	}
}

func TestShouldRetrieveKnowledgeModes(t *testing.T) {
	tests := []struct {
		name  string
		query string
		mode  KnowledgeMode
		want  bool
	}{
		{name: "auto normal chat", query: "hello", mode: KnowledgeAuto, want: false},
		{name: "auto alert query", query: "\u73b0\u5728\u6709\u54ea\u4e9b\u544a\u8b66\u9700\u8981\u5904\u7406", mode: KnowledgeAuto, want: true},
		{name: "auto runbook query", query: "\u5e2e\u6211\u6309\u544a\u8b66\u5904\u7406\u624b\u518c\u6392\u67e5", mode: KnowledgeAuto, want: true},
		{name: "auto explicit off", query: "\u4e0d\u67e5\u77e5\u8bc6\u5e93\uff0c\u53ea\u6839\u636e\u5f53\u524d\u5bf9\u8bdd\u56de\u7b54", mode: KnowledgeAuto, want: false},
		{name: "auto english incident query", query: "troubleshoot this timeout incident", mode: KnowledgeAuto, want: true},
		{name: "auto short english keyword", query: "log", mode: KnowledgeAuto, want: true},
		{name: "always empty query", query: " ", mode: KnowledgeAlways, want: false},
		{name: "always normal query", query: "hello", mode: KnowledgeAlways, want: true},
		{name: "never document query", query: "show the manual", mode: KnowledgeNever, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetrieveKnowledge(tt.query, tt.mode, false); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestShouldRetrieveKnowledgeHonorsPreprocessedNeed(t *testing.T) {
	if !shouldRetrieveKnowledge("hello", KnowledgeAuto, true) {
		t.Fatal("expected preprocessed knowledge need to trigger retrieval in auto mode")
	}
	if shouldRetrieveKnowledge("show the manual", KnowledgeNever, true) {
		t.Fatal("expected never mode to suppress preprocessed knowledge need")
	}
}

func TestMergeNormalizedPhrasesDeduplicatesConfigKeywords(t *testing.T) {
	got := mergeNormalizedPhrases(
		[]string{" alert ", "\u6545\u969c"},
		[]string{"ALERT", "\u652f\u4ed8\u56de\u8c03\u5931\u8d25", ""},
	)

	want := []string{"alert", "\u6545\u969c", "\u652f\u4ed8\u56de\u8c03\u5931\u8d25"}
	if len(got) != len(want) {
		t.Fatalf("expected %d phrases, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected phrase %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestAssembleModelContextAppliesContextBudget(t *testing.T) {
	input := &UserMessage{
		Query:         "hello",
		Summary:       strings.Repeat("s", SummaryMaxChars+20),
		ToolResults:   strings.Repeat("t", ToolResultsMaxChars+20),
		DurableMemory: strings.Repeat("d", DurableMemoryMaxChars+20),
	}

	got := AssembleModelContext(context.Background(), input, time.Now())
	if utf8.RuneCountInString(got.Summary) != SummaryMaxChars {
		t.Fatalf("expected summary budget %d, got %d", SummaryMaxChars, utf8.RuneCountInString(got.Summary))
	}
	if utf8.RuneCountInString(got.ToolResults) != ToolResultsMaxChars {
		t.Fatalf("expected tool results budget %d, got %d", ToolResultsMaxChars, utf8.RuneCountInString(got.ToolResults))
	}
	if utf8.RuneCountInString(got.DurableMemory) != DurableMemoryMaxChars {
		t.Fatalf("expected durable memory budget %d, got %d", DurableMemoryMaxChars, utf8.RuneCountInString(got.DurableMemory))
	}
}

func TestAssembleModelContextUsesCleanedInput(t *testing.T) {
	got := AssembleModelContext(context.Background(), &UserMessage{Query: " 支付\n\t回调失败 "}, time.Now())
	if got.Content != "支付 回调失败" {
		t.Fatalf("expected cleaned content, got %q", got.Content)
	}
	if got.KnowledgeQuery != "支付 回调失败" {
		t.Fatalf("expected cleaned knowledge query, got %q", got.KnowledgeQuery)
	}
	if !strings.Contains(got.RequestAnalysis, "Domain: payment") {
		t.Fatalf("expected payment analysis, got %q", got.RequestAnalysis)
	}
}

func TestBudgetHistoryMessagesKeepsRecentUserAssistantPair(t *testing.T) {
	history := []*schema.Message{
		schema.UserMessage("old question"),
		schema.AssistantMessage("old answer", nil),
		schema.UserMessage("recent question"),
		schema.AssistantMessage("recent answer", nil),
	}

	got := budgetHistoryMessages(history, len("recent question")+len("recent answer"))
	if len(got) != 2 {
		t.Fatalf("expected one recent pair, got %d messages", len(got))
	}
	if got[0].Content != "recent question" || got[1].Content != "recent answer" {
		t.Fatalf("expected recent pair, got %q / %q", got[0].Content, got[1].Content)
	}
	if !isUserMessage(got[0]) {
		t.Fatalf("expected history to start with a user message, got role=%s", got[0].Role)
	}
}

func TestBudgetDocumentsCapsTotalAndPerDocumentChars(t *testing.T) {
	docs := []*schema.Document{
		{Content: strings.Repeat("a", 10)},
		{Content: strings.Repeat("b", 10)},
		{Content: strings.Repeat("c", 10)},
	}

	got := budgetDocuments(docs, 12, 5)
	if len(got) != 3 {
		t.Fatalf("expected three truncated docs within total budget, got %d", len(got))
	}
	total := 0
	for _, doc := range got {
		docLen := utf8.RuneCountInString(doc.Content)
		if docLen > 5 {
			t.Fatalf("expected per-doc budget <= 5, got %d", docLen)
		}
		total += docLen
	}
	if total > 12 {
		t.Fatalf("expected total document budget <= 12, got %d", total)
	}
	if docs[0].Content != strings.Repeat("a", 10) {
		t.Fatal("expected original document to remain unchanged")
	}
}

func TestContextDocumentsSkipsKnowledgeForNormalChat(t *testing.T) {
	got, err := newContextDocumentsLambda(context.Background(), &UserMessage{Query: "hello"})
	if err != nil {
		t.Fatalf("expected normal chat context to skip retrieval, got err=%v", err)
	}

	docs := toDocuments(got["documents"])
	if len(docs) != 0 {
		t.Fatalf("expected no knowledge documents, got %d", len(docs))
	}
}

func TestInputToChatLoadsProjectContext(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("Use project evidence rules."), 0o644); err != nil {
		t.Fatalf("write AGENT.md: %v", err)
	}
	if err = os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		if restoreErr := os.Chdir(wd); restoreErr != nil {
			t.Fatalf("restore wd: %v", restoreErr)
		}
	}()

	got, err := newInputToChatLambda(context.Background(), &UserMessage{Query: "hello"})
	if err != nil {
		t.Fatalf("expected input conversion success, got err=%v", err)
	}
	if got["project_context"] != "Use project evidence rules." {
		t.Fatalf("expected project context, got %#v", got["project_context"])
	}
}
