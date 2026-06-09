package chat_pipeline

import (
	"SREagent/internal/ai/agent/trace"
	"SREagent/internal/ai/evidence"
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func EnforceEvidenceAnswer(ctx context.Context, answer string, steps []trace.AgentStep) (string, error) {
	available := evidenceIDsFromTrace(steps)
	validation := evidence.ValidateAnswer(answer, available)
	if validation.OK {
		return answer, nil
	}
	if len(available.EvidenceIDs) == 0 && len(available.FailedQueryIDs) == 0 {
		return answer, nil
	}
	repaired, err := repairEvidenceAnswer(ctx, answer, steps, validation.Reasons)
	if err != nil {
		return conservativeEvidenceAnswer(available, validation.Reasons), nil
	}
	if evidence.ValidateAnswer(repaired, available).OK {
		return repaired, nil
	}
	return conservativeEvidenceAnswer(available, validation.Reasons), nil
}

func evidenceIDsFromTrace(steps []trace.AgentStep) evidence.EvidenceSet {
	sets := make([]evidence.EvidenceSet, 0, len(steps))
	for _, step := range steps {
		if step.Phase != "end" && step.Phase != "error" {
			continue
		}
		sets = append(sets, evidence.ExtractIDs(step.Output), evidence.ExtractIDs(step.Error))
	}
	return evidence.Merge(sets...)
}

func repairEvidenceAnswer(ctx context.Context, answer string, steps []trace.AgentStep, reasons []string) (string, error) {
	model, err := newChatModel(ctx)
	if err != nil {
		return "", err
	}
	msg, err := model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(`Rewrite the assistant answer to obey evidence rules.
Only cite successful evidence_id values as evidence.
failed_query_id values are evidence gaps, not supporting evidence.
If a fact lacks evidence, mark it as 未验证/证据缺口.
Do not invent metrics, logs, or tool results.`),
		schema.UserMessage("Validation failures:\n" + strings.Join(reasons, "\n") + "\n\nTool trace:\n" + compactSteps(steps) + "\n\nOriginal answer:\n" + answer),
	})
	if err != nil {
		return "", err
	}
	return msg.Content, nil
}

func compactSteps(steps []trace.AgentStep) string {
	var b strings.Builder
	for _, step := range steps {
		if step.Phase != "end" && step.Phase != "error" {
			continue
		}
		b.WriteString(step.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func conservativeEvidenceAnswer(available evidence.EvidenceSet, reasons []string) string {
	var b strings.Builder
	b.WriteString("已验证证据：\n")
	if len(available.EvidenceIDs) == 0 {
		b.WriteString("- 暂无可用于支持结论的成功证据。\n")
	} else {
		for id := range available.EvidenceIDs {
			b.WriteString("- ")
			b.WriteString(id)
			b.WriteString("：工具调用成功，但原回答未能按证据规则可靠引用。\n")
		}
	}
	b.WriteString("\n未验证/证据缺口：\n")
	if len(available.FailedQueryIDs) == 0 {
		b.WriteString("- 未发现失败查询 ID。\n")
	} else {
		for id := range available.FailedQueryIDs {
			b.WriteString("- ")
			b.WriteString(id)
			b.WriteString("：对应工具调用失败、超时或无权限，不能作为事实证据。\n")
		}
	}
	b.WriteString("\n判断：\n- 当前回答未通过证据校验，不能给出确认性诊断结论。\n")
	if len(reasons) > 0 {
		b.WriteString("\n校验原因：\n")
		for _, reason := range reasons {
			b.WriteString("- ")
			b.WriteString(reason)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n下一步：\n- 重新执行失败查询或补充成功证据后再确认根因。")
	return b.String()
}
