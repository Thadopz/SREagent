package eval

import (
	"SREagent/internal/ai/agent/chat_pipeline"
	"SREagent/internal/ai/agent/trace"
	"time"
)

type Mode string

const (
	ModeMock  Mode = "mock"
	ModeSmoke Mode = "smoke"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type MockContext struct {
	Summary       string        `json:"summary,omitempty"`
	SessionState  string        `json:"session_state,omitempty"`
	DurableMemory string        `json:"durable_memory,omitempty"`
	ToolResults   string        `json:"tool_results,omitempty"`
	Documents     []DocEvidence `json:"documents,omitempty"`
	ToolOutputs   []ToolFixture `json:"tool_outputs,omitempty"`
	MockAnswer    string        `json:"mock_answer,omitempty"`
}

type ToolFixture struct {
	Name   string `json:"name"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type Expected struct {
	UseKnowledge      *bool    `json:"use_knowledge,omitempty"`
	Tools             []string `json:"tools,omitempty"`
	NotTools          []string `json:"not_tools,omitempty"`
	AnswerContains    []string `json:"answer_contains,omitempty"`
	AnswerNotContains []string `json:"answer_not_contains,omitempty"`
	EvidenceContains  []string `json:"evidence_contains,omitempty"`
}

type Case struct {
	ID          string        `json:"id"`
	Category    string        `json:"category"`
	Query       string        `json:"query"`
	History     []ChatMessage `json:"history,omitempty"`
	MockContext MockContext   `json:"mock_context,omitempty"`
	Expected    Expected      `json:"expected,omitempty"`
}

type ContextEvidence struct {
	SummaryChars       int  `json:"summary_chars"`
	SessionStateChars  int  `json:"session_state_chars"`
	DurableMemoryChars int  `json:"durable_memory_chars"`
	ToolResultsChars   int  `json:"tool_results_chars"`
	HistoryMessages    int  `json:"history_messages"`
	UseKnowledge       bool `json:"use_knowledge"`
	PromptContextChars int  `json:"prompt_context_chars"`
}

type DocEvidence struct {
	Source  string         `json:"source,omitempty"`
	Content string         `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
	Stage   string         `json:"stage,omitempty"`
}

type ToolEvidence struct {
	Name    string `json:"name"`
	Input   string `json:"input,omitempty"`
	Output  string `json:"output,omitempty"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Elapsed int64  `json:"elapsed_ms,omitempty"`
}

type EvalScores struct {
	RouteScore    float64  `json:"route_score"`
	EvidenceScore float64  `json:"evidence_score"`
	AnswerScore   float64  `json:"answer_score"`
	SafetyScore   float64  `json:"safety_score"`
	Pass          bool     `json:"pass"`
	Reasons       []string `json:"reasons,omitempty"`
}

type EvalCaseResult struct {
	ID             string            `json:"id"`
	Category       string            `json:"category"`
	Mode           Mode              `json:"mode"`
	Query          string            `json:"query"`
	ConversationID string            `json:"conversation_id"`
	Context        ContextEvidence   `json:"context"`
	RetrievedDocs  []DocEvidence     `json:"retrieved_docs,omitempty"`
	ToolCalls      []ToolEvidence    `json:"tool_calls,omitempty"`
	TraceSteps     []trace.AgentStep `json:"trace_steps,omitempty"`
	Answer         string            `json:"answer"`
	Error          string            `json:"error,omitempty"`
	ElapsedMS      int64             `json:"elapsed_ms"`
	Scores         EvalScores        `json:"scores"`
	StartedAt      time.Time         `json:"started_at"`
}

type RunSummary struct {
	Mode           Mode           `json:"mode"`
	Total          int            `json:"total"`
	Passed         int            `json:"passed"`
	Failed         int            `json:"failed"`
	PassRate       float64        `json:"pass_rate"`
	ToolCallCounts map[string]int `json:"tool_call_counts"`
	KnowledgeUsed  int            `json:"knowledge_used"`
	FailureReasons []string       `json:"failure_reasons,omitempty"`
}

type RunOptions struct {
	Mode               Mode
	CasesPath          string
	ReportsDir         string
	RunID              string
	CaseTimeoutSeconds int
}

type RunResult struct {
	Summary      RunSummary
	Results      []EvalCaseResult
	JSONLPath    string
	MarkdownPath string
}

func docFromPipelineDoc(doc chat_pipeline.RetrievedDocEvidence) DocEvidence {
	return DocEvidence{
		Source:  doc.Source,
		Content: doc.Content,
		Meta:    doc.Meta,
		Stage:   doc.Stage,
	}
}
