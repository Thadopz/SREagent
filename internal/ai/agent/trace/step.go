package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
)

type AgentStep struct {
	Index     int       `json:"index"`
	Agent     string    `json:"agent"`
	Phase     string    `json:"phase"`
	Component string    `json:"component,omitempty"`
	Type      string    `json:"type,omitempty"`
	Name      string    `json:"name,omitempty"`
	Input     string    `json:"input,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	ElapsedMS int64     `json:"elapsed_ms,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s AgentStep) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("step=%d agent=%s phase=%s name=%s", s.Index, s.Agent, s.Phase, s.Name)
	}
	return string(b)
}

type Recorder struct {
	mu    sync.Mutex
	steps []AgentStep
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Record(step AgentStep) AgentStep {
	if r == nil {
		return step
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	step.Index = len(r.steps) + 1
	if step.CreatedAt.IsZero() {
		step.CreatedAt = time.Now()
	}
	r.steps = append(r.steps, step)
	return step
}

func (r *Recorder) Steps() []AgentStep {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	steps := make([]AgentStep, len(r.steps))
	copy(steps, r.steps)
	return steps
}

func (r *Recorder) Strings() []string {
	steps := r.Steps()
	res := make([]string, 0, len(steps))
	for _, step := range steps {
		res = append(res, step.String())
	}
	return res
}

func Callback(agent string, recorder *Recorder) callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()

	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		recorder.Record(AgentStep{
			Agent:     agent,
			Phase:     "start",
			Component: string(info.Component),
			Type:      info.Type,
			Name:      info.Name,
			Input:     compactJSON(input),
		})
		return ctx
	})

	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		recorder.Record(AgentStep{
			Agent:     agent,
			Phase:     "end",
			Component: string(info.Component),
			Type:      info.Type,
			Name:      info.Name,
			Output:    compactJSON(output),
		})
		return ctx
	})

	return builder.Build()
}

func StreamingCallback(agent string, recorder *Recorder, emit func(event string, step AgentStep)) callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()

	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		if info == nil {
			return ctx
		}
		step := recorder.Record(AgentStep{
			Agent:     agent,
			Phase:     "start",
			Component: string(info.Component),
			Type:      info.Type,
			Name:      info.Name,
			Input:     compactJSON(input),
		})
		emitTraceStep(info, step, emit)
		return ctx
	})

	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		if info == nil {
			return ctx
		}
		step := recorder.Record(AgentStep{
			Agent:     agent,
			Phase:     "end",
			Component: string(info.Component),
			Type:      info.Type,
			Name:      info.Name,
			Output:    compactJSON(output),
		})
		emitTraceStep(info, step, emit)
		return ctx
	})

	builder.OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		if info == nil {
			return ctx
		}
		errText := ""
		if err != nil {
			errText = err.Error()
		}
		step := recorder.Record(AgentStep{
			Agent:     agent,
			Phase:     "error",
			Component: string(info.Component),
			Type:      info.Type,
			Name:      info.Name,
			Error:     truncate(errText, 1000),
		})
		emitTraceStep(info, step, emit)
		return ctx
	})

	return builder.Build()
}

func emitTraceStep(info *callbacks.RunInfo, step AgentStep, emit func(event string, step AgentStep)) {
	if emit == nil || info == nil {
		return
	}
	if info.Component != components.ComponentOfTool {
		return
	}
	switch step.Phase {
	case "start":
		emit("tool_start", step)
	case "end":
		emit("tool_result", step)
	case "error":
		emit("tool_error", step)
	}
}

func compactJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return truncate(string(b), 2000)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
