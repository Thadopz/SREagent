package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cases.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"c1","category":"chat","query":"hello"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cases, err := LoadCases(path)
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	if len(cases) != 1 || cases[0].ID != "c1" {
		t.Fatalf("unexpected cases: %#v", cases)
	}
}

func TestScoreCaseDetectsFailures(t *testing.T) {
	useKnowledge := true
	result := EvalCaseResult{
		Answer:  "hello",
		Context: ContextEvidence{UseKnowledge: false},
	}
	score := ScoreCase(Case{Expected: Expected{
		UseKnowledge:   &useKnowledge,
		Tools:          []string{"query_prometheus_alerts"},
		AnswerContains: []string{"incident"},
	}}, result)
	if score.Pass {
		t.Fatal("expected score to fail")
	}
	if len(score.Reasons) != 3 {
		t.Fatalf("expected three failure reasons, got %v", score.Reasons)
	}
}

func TestRunCaseMockCollectsEvidenceAndScores(t *testing.T) {
	useKnowledge := true
	c := Case{
		ID:       "knowledge",
		Category: "knowledge",
		Query:    "how to handle alert by runbook",
		MockContext: MockContext{
			Documents:  []DocEvidence{{Source: "runbook.md", Content: "runbook evidence"}},
			MockAnswer: "runbook answer",
		},
		Expected: Expected{
			UseKnowledge:     &useKnowledge,
			AnswerContains:   []string{"runbook"},
			EvidenceContains: []string{"runbook evidence"},
		},
	}

	result, err := RunCase(context.Background(), c, RunOptions{Mode: ModeMock, RunID: "test"})
	if err != nil {
		t.Fatalf("run case: %v", err)
	}
	if !result.Scores.Pass {
		t.Fatalf("expected pass, got reasons=%v result=%#v", result.Scores.Reasons, result)
	}
	if len(result.RetrievedDocs) == 0 {
		t.Fatal("expected retrieved docs")
	}
}

func TestWriteReports(t *testing.T) {
	dir := t.TempDir()
	results := []EvalCaseResult{{
		ID:     "c1",
		Mode:   ModeMock,
		Answer: "ok",
		Scores: EvalScores{Pass: true},
	}}
	runResult, err := WriteReports(results, RunOptions{Mode: ModeMock, ReportsDir: dir, RunID: "test"})
	if err != nil {
		t.Fatalf("write reports: %v", err)
	}
	if _, err := os.Stat(runResult.JSONLPath); err != nil {
		t.Fatalf("missing jsonl report: %v", err)
	}
	if _, err := os.Stat(runResult.MarkdownPath); err != nil {
		t.Fatalf("missing markdown report: %v", err)
	}
}
