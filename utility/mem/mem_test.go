package mem

import (
	"SREagent/internal/ai/evidence"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	toolcomponent "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

func TestGetMessagesReturnsCopy(t *testing.T) {
	m := &SimpleMemory{MaxWindowSize: 20}
	m.SetMessages(schema.UserMessage("hello"))

	msgs := m.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	msgs[0].Content = "changed"
	again := m.GetMessages()
	if again[0].Content != "hello" {
		t.Fatalf("expected internal message to remain unchanged, got %q", again[0].Content)
	}
}

func TestGetContextMessagesLimitsMessageCount(t *testing.T) {
	m := &SimpleMemory{MaxWindowSize: 20}
	for i := 0; i < 6; i++ {
		m.SetMessages(schema.UserMessage("q"))
		m.SetMessages(schema.AssistantMessage("a", nil))
	}

	msgs := m.GetContextMessages(ContextPolicy{MaxMessages: 4, MaxChars: 1000})
	if len(msgs) != 4 {
		t.Fatalf("expected 4 context messages, got %d", len(msgs))
	}
}

func TestGetContextMessagesLimitsCharBudget(t *testing.T) {
	m := &SimpleMemory{MaxWindowSize: 20}
	m.SetMessages(schema.UserMessage("old-question"))
	m.SetMessages(schema.AssistantMessage("old-answer", nil))
	m.SetMessages(schema.UserMessage("new-question"))
	m.SetMessages(schema.AssistantMessage("new-answer", nil))

	msgs := m.GetContextMessages(ContextPolicy{MaxMessages: 10, MaxChars: len("new-question") + len("new-answer") + 1})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 newest paired messages under tight char budget, got %d", len(msgs))
	}
	if msgs[0].Content != "new-question" || msgs[1].Content != "new-answer" {
		t.Fatalf("expected newest pair, got %#v", msgs)
	}
}

func TestStoredMessagesAreTrimmedInPairs(t *testing.T) {
	m := &SimpleMemory{MaxWindowSize: 4}
	for i := 0; i < 4; i++ {
		m.SetMessages(schema.UserMessage("q"))
		m.SetMessages(schema.AssistantMessage("a", nil))
	}

	msgs := m.GetMessages()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 stored messages, got %d", len(msgs))
	}
}

func TestTrimmedMessagesAreMergedIntoSummary(t *testing.T) {
	m := &SimpleMemory{MaxWindowSize: 4}
	m.SetMessages(schema.UserMessage("first-question"))
	m.SetMessages(schema.AssistantMessage("first-answer", nil))
	m.SetMessages(schema.UserMessage("second-question"))
	m.SetMessages(schema.AssistantMessage("second-answer", nil))
	m.SetMessages(schema.UserMessage("third-question"))
	m.SetMessages(schema.AssistantMessage("third-answer", nil))

	summary := m.GetSummary()
	if !strings.Contains(summary, "first-question") || !strings.Contains(summary, "first-answer") {
		t.Fatalf("expected summary to contain trimmed pair, got %q", summary)
	}

	msgs := m.GetMessages()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 retained messages, got %d", len(msgs))
	}
	if msgs[0].Content != "second-question" {
		t.Fatalf("expected second pair to be oldest retained message, got %q", msgs[0].Content)
	}
}

func TestSummaryIsTrimmedToBudget(t *testing.T) {
	m := &SimpleMemory{MaxWindowSize: 2}
	for i := 0; i < 40; i++ {
		m.SetMessages(schema.UserMessage(strings.Repeat("x", 300)))
		m.SetMessages(schema.AssistantMessage(strings.Repeat("y", 300), nil))
	}

	if got := len([]rune(m.GetSummary())); got > defaultSummaryMaxChars {
		t.Fatalf("expected summary <= %d runes, got %d", defaultSummaryMaxChars, got)
	}
}

func TestSimpleMemoryPersistsMessagesAndSummary(t *testing.T) {
	restore := setMemoryStoreForTest(newFileMemoryStore(filepath.Join(t.TempDir(), "memory")))
	defer restore()

	memory := GetSimpleMemory("session-1")
	memory.MaxWindowSize = 2
	memory.SetMessages(schema.UserMessage("first-question"))
	memory.SetMessages(schema.AssistantMessage("first-answer", nil))
	memory.SetMessages(schema.UserMessage("second-question"))
	memory.SetMessages(schema.AssistantMessage("second-answer", nil))

	mu.Lock()
	SimpleMemoryMap = make(map[string]*SimpleMemory)
	mu.Unlock()

	reloaded := GetSimpleMemory("session-1")
	msgs := reloaded.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected persisted retained messages, got %d", len(msgs))
	}
	if msgs[0].Content != "second-question" || msgs[1].Content != "second-answer" {
		t.Fatalf("expected newest persisted pair, got %#v", msgs)
	}
	if summary := reloaded.GetSummary(); !strings.Contains(summary, "first-question") {
		t.Fatalf("expected persisted summary, got %q", summary)
	}
}

func TestConversationMemoryIsScopedByUserAndConversation(t *testing.T) {
	restore := setMemoryStoreForTest(newFileMemoryStore(filepath.Join(t.TempDir(), "memory")))
	defer restore()

	alice := GetConversationMemory("alice", "shared-conversation")
	bob := GetConversationMemory("bob", "shared-conversation")
	alice.SetMessages(schema.UserMessage("alice question"))
	bob.SetMessages(schema.UserMessage("bob question"))

	if got := alice.GetMessages()[0].Content; got != "alice question" {
		t.Fatalf("expected alice memory to stay isolated, got %q", got)
	}
	if got := bob.GetMessages()[0].Content; got != "bob question" {
		t.Fatalf("expected bob memory to stay isolated, got %q", got)
	}

	mu.Lock()
	SimpleMemoryMap = make(map[string]*SimpleMemory)
	mu.Unlock()

	reloadedAlice := GetConversationMemory("alice", "shared-conversation")
	reloadedBob := GetConversationMemory("bob", "shared-conversation")
	if got := reloadedAlice.GetMessages()[0].Content; got != "alice question" {
		t.Fatalf("expected persisted alice memory to stay isolated, got %q", got)
	}
	if got := reloadedBob.GetMessages()[0].Content; got != "bob question" {
		t.Fatalf("expected persisted bob memory to stay isolated, got %q", got)
	}
}

func TestMySQLMemoryStorePersistsMessagesAndSummary(t *testing.T) {
	if os.Getenv("SUPERBIZ_MYSQL_MEMORY_TEST") != "1" {
		t.Skip("set SUPERBIZ_MYSQL_MEMORY_TEST=1 to run MySQL memory store integration test")
	}

	gdb.SetConfigGroup(gdb.DefaultGroupName, gdb.ConfigGroup{{
		Type: "mysql",
		Host: "127.0.0.1",
		Port: "3306",
		User: "root",
		Pass: "123456",
		Name: "superbiz_agent",
	}})

	id := "mysql-memory-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() {
		_, _ = g.DB().Model("conversation_turns").Where("conversation_id", id).Delete()
		_, _ = g.DB().Model("conversation_summaries").Where("conversation_id", id).Delete()
		_, _ = g.DB().Model("session_states").Where("conversation_id", id).Delete()
		_, _ = g.DB().Model("durable_memories").Where("user_id", id).Delete()
		_, _ = g.DB().Model("tool_results").Where("conversation_id", id).Delete()
		_, _ = g.DB().Model("conversations").Where("conversation_id", id).Delete()
	})

	mysqlStore := newMySQLMemoryStore()
	snapshot := &memorySnapshot{
		ID:            id,
		Summary:       "persisted summary",
		MaxWindowSize: 2,
		UpdatedAt:     time.Now(),
	}
	if err := mysqlStore.Append(snapshot, schema.UserMessage("persisted question")); err != nil {
		t.Fatalf("append mysql user turn: %v", err)
	}
	if err := mysqlStore.Append(snapshot, schema.AssistantMessage("persisted answer", nil)); err != nil {
		t.Fatalf("append mysql assistant turn: %v", err)
	}

	got, err := mysqlStore.Load(id)
	if err != nil {
		t.Fatalf("load mysql memory snapshot: %v", err)
	}
	if got == nil || len(got.Messages) != 2 {
		t.Fatalf("expected persisted mysql messages, got %#v", got)
	}
	if got.Messages[0].Content != "persisted question" || got.Messages[1].Content != "persisted answer" {
		t.Fatalf("expected persisted mysql turn contents, got %#v", got.Messages)
	}
	if got.Summary != "persisted summary" {
		t.Fatalf("expected persisted mysql summary, got %q", got.Summary)
	}

	if err = UpdateSessionState(context.Background(), id, id, "investigate alert", "query the logs first"); err != nil {
		t.Fatalf("write mysql session state: %v", err)
	}
	if err = UpdateSessionState(context.Background(), id, id, "follow-up question", "logs show a timeout"); err != nil {
		t.Fatalf("update mysql session state: %v", err)
	}
	if _, err = g.DB().Model("durable_memories").Data(g.Map{
		"user_id": id,
		"kind":    "preference",
		"content": "keep incident summaries short",
		"status":  "active",
	}).Insert(); err != nil {
		t.Fatalf("insert durable memory fixture: %v", err)
	}
	if err = SaveToolResult(context.Background(), id, id, ToolResult{
		ToolName:      "query_log",
		InputJSON:     `{"query":"timeout"}`,
		OutputSummary: "found 3 timeout rows",
		OutputJSON:    `{"success":true,"message":"found 3 timeout rows"}`,
		Status:        "success",
	}); err != nil {
		t.Fatalf("save tool result fixture: %v", err)
	}
	handler := ToolResultCallback(id, id)
	info := &callbacks.RunInfo{
		Component: components.ComponentOfTool,
		Name:      "query_internal_docs",
	}
	callbackCtx := handler.OnStart(context.Background(), info, &toolcomponent.CallbackInput{
		ArgumentsInJSON: `{"query":"alert runbook"}`,
	})
	handler.OnEnd(callbackCtx, info, &toolcomponent.CallbackOutput{
		Response: `{"success":true,"message":"retrieved 2 docs"}`,
	})

	layers, err := LoadContextLayers(context.Background(), id, id)
	if err != nil {
		t.Fatalf("load mysql context layers: %v", err)
	}
	if !strings.Contains(layers.SessionState, "Goal: investigate alert") {
		t.Fatalf("expected mysql session state layer, got %q", layers.SessionState)
	}
	if !strings.Contains(layers.SessionState, "follow-up question") {
		t.Fatalf("expected mysql session payload update, got %q", layers.SessionState)
	}
	if !strings.Contains(layers.DurableMemory, "keep incident summaries short") {
		t.Fatalf("expected mysql durable memory layer, got %q", layers.DurableMemory)
	}
	if !strings.Contains(layers.ToolResults, "query_log [success]: found 3 timeout rows") {
		t.Fatalf("expected mysql tool result layer, got %q", layers.ToolResults)
	}
	if !strings.Contains(layers.ToolResults, "query_internal_docs [success]") {
		t.Fatalf("expected callback-captured tool result layer, got %q", layers.ToolResults)
	}
}

func TestFormatContextLayers(t *testing.T) {
	state := formatSessionState(sessionStateRow{
		Goal:   "investigate alert",
		Status: "active",
		State:  `{"next_action":"query logs"}`,
	})
	if !strings.Contains(state, "Goal: investigate alert") || !strings.Contains(state, "State:") {
		t.Fatalf("expected formatted session state, got %q", state)
	}

	durable := formatDurableMemories([]durableMemoryRow{
		{Kind: "preference", Content: "keep incident summaries short"},
		{Content: "production changes need confirmation"},
	})
	if !strings.Contains(durable, "- preference: keep incident summaries short") {
		t.Fatalf("expected formatted durable preference, got %q", durable)
	}
	if !strings.Contains(durable, "- memory: production changes need confirmation") {
		t.Fatalf("expected default durable memory kind, got %q", durable)
	}
}

func TestTrimSessionStateText(t *testing.T) {
	got := trimSessionStateText(" first line\nsecond line ", 6)
	if got != "first " {
		t.Fatalf("expected newline normalization and trim budget, got %q", got)
	}
}

func TestFormatToolResults(t *testing.T) {
	got := formatToolResults([]toolResultRow{
		{ToolName: "query_log", OutputSummary: "newer result", Status: "success"},
		{ToolName: "query_internal_docs", OutputSummary: "older result", Status: "success"},
	})
	if got != "- query_internal_docs [success]: older result\n- query_log [success]: newer result" {
		t.Fatalf("expected chronological tool results, got %q", got)
	}
}

func TestFormatEnvelopeSummarySeparatesEvidenceAndFailedQuery(t *testing.T) {
	success := formatEnvelopeSummary(evidence.Envelope{
		Status:           evidence.StatusSuccess,
		EvidenceID:       "ev_1_abcd1234",
		CanUseAsEvidence: true,
		DataStatus:       evidence.DataPresent,
		Summary:          "query returned rows",
	}, "")
	if !strings.Contains(success, "evidence_id=ev_1_abcd1234") {
		t.Fatalf("expected evidence id in summary, got %q", success)
	}

	failed := formatEnvelopeSummary(evidence.Envelope{
		Status:        evidence.StatusError,
		FailedQueryID: "fq_1_abcd1234",
		DataStatus:    evidence.DataUnknown,
		Summary:       "query failed",
	}, "")
	if !strings.Contains(failed, "failed_query_id=fq_1_abcd1234") {
		t.Fatalf("expected failed query id in summary, got %q", failed)
	}
}
