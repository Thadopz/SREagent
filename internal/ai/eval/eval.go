package eval

import (
	"context"
	"time"
)

func Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	if opts.Mode == "" {
		opts.Mode = ModeMock
	}
	if opts.ReportsDir == "" {
		opts.ReportsDir = "reports"
	}
	if opts.RunID == "" {
		opts.RunID = time.Now().Format("20060102-150405")
	}
	if opts.CaseTimeoutSeconds <= 0 {
		opts.CaseTimeoutSeconds = 60
	}
	cases, err := LoadCases(opts.CasesPath)
	if err != nil {
		return RunResult{}, err
	}
	results, err := RunCases(ctx, cases, opts)
	if err != nil {
		return RunResult{}, err
	}
	return WriteReports(results, opts)
}
