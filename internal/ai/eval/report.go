package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Summarize(mode Mode, results []EvalCaseResult) RunSummary {
	summary := RunSummary{
		Mode:           mode,
		Total:          len(results),
		ToolCallCounts: map[string]int{},
	}
	reasonSet := map[string]bool{}
	for _, result := range results {
		if result.Scores.Pass {
			summary.Passed++
		} else {
			summary.Failed++
		}
		if result.Context.UseKnowledge {
			summary.KnowledgeUsed++
		}
		for _, toolCall := range result.ToolCalls {
			if toolCall.Name != "" {
				summary.ToolCallCounts[toolCall.Name]++
			}
		}
		for _, reason := range result.Scores.Reasons {
			if !reasonSet[reason] {
				reasonSet[reason] = true
				summary.FailureReasons = append(summary.FailureReasons, reason)
			}
		}
	}
	if summary.Total > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.Total)
	}
	sort.Strings(summary.FailureReasons)
	return summary
}

func WriteReports(results []EvalCaseResult, opts RunOptions) (RunResult, error) {
	if opts.ReportsDir == "" {
		opts.ReportsDir = "reports"
	}
	if opts.RunID == "" {
		opts.RunID = "eval"
	}
	if err := os.MkdirAll(opts.ReportsDir, 0755); err != nil {
		return RunResult{}, err
	}

	jsonlPath := filepath.Join(opts.ReportsDir, "eval-"+opts.RunID+".jsonl")
	mdPath := filepath.Join(opts.ReportsDir, "eval-"+opts.RunID+".md")
	if err := writeJSONL(jsonlPath, results); err != nil {
		return RunResult{}, err
	}
	summary := Summarize(opts.Mode, results)
	if err := os.WriteFile(mdPath, []byte(markdownReport(summary, results)), 0644); err != nil {
		return RunResult{}, err
	}
	return RunResult{Summary: summary, Results: results, JSONLPath: jsonlPath, MarkdownPath: mdPath}, nil
}

func writeJSONL(path string, results []EvalCaseResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, result := range results {
		if err := enc.Encode(result); err != nil {
			return err
		}
	}
	return w.Flush()
}

func markdownReport(summary RunSummary, results []EvalCaseResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Eval Report\n\n")
	fmt.Fprintf(&b, "- Mode: `%s`\n", summary.Mode)
	fmt.Fprintf(&b, "- Total: %d\n", summary.Total)
	fmt.Fprintf(&b, "- Passed: %d\n", summary.Passed)
	fmt.Fprintf(&b, "- Failed: %d\n", summary.Failed)
	fmt.Fprintf(&b, "- Pass rate: %.2f%%\n", summary.PassRate*100)
	fmt.Fprintf(&b, "- Knowledge used: %d\n\n", summary.KnowledgeUsed)

	if len(summary.ToolCallCounts) > 0 {
		fmt.Fprintf(&b, "## Tool Calls\n\n")
		names := make([]string, 0, len(summary.ToolCallCounts))
		for name := range summary.ToolCallCounts {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(&b, "- `%s`: %d\n", name, summary.ToolCallCounts[name])
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Cases\n\n")
	for _, result := range results {
		status := "PASS"
		if !result.Scores.Pass {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "### %s `%s`\n\n", status, result.ID)
		fmt.Fprintf(&b, "- Category: `%s`\n", result.Category)
		fmt.Fprintf(&b, "- Knowledge: `%v`\n", result.Context.UseKnowledge)
		fmt.Fprintf(&b, "- Tools: `%s`\n", strings.Join(toolNames(result.ToolCalls), ", "))
		fmt.Fprintf(&b, "- Elapsed: %dms\n", result.ElapsedMS)
		if result.Error != "" {
			fmt.Fprintf(&b, "- Error: %s\n", result.Error)
		}
		if len(result.Scores.Reasons) > 0 {
			fmt.Fprintf(&b, "- Reasons: %s\n", strings.Join(result.Scores.Reasons, "; "))
		}
		fmt.Fprintf(&b, "- Answer: %s\n\n", truncateText(result.Answer, 400))
	}
	return b.String()
}

func toolNames(calls []ToolEvidence) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		if call.Name != "" {
			names = append(names, call.Name)
		}
	}
	return names
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
