package evidence

import (
	"regexp"
	"strings"
)

var (
	evidenceIDPattern    = regexp.MustCompile(`\bev_[0-9]+_[a-f0-9]+\b`)
	failedQueryIDPattern = regexp.MustCompile(`\bfq_[0-9]+_[a-f0-9]+\b`)
)

type EvidenceSet struct {
	EvidenceIDs    map[string]bool
	FailedQueryIDs map[string]bool
}

type ValidationResult struct {
	OK      bool
	Reasons []string
}

func ExtractIDs(text string) EvidenceSet {
	set := EvidenceSet{
		EvidenceIDs:    map[string]bool{},
		FailedQueryIDs: map[string]bool{},
	}
	for _, id := range evidenceIDPattern.FindAllString(text, -1) {
		set.EvidenceIDs[id] = true
	}
	for _, id := range failedQueryIDPattern.FindAllString(text, -1) {
		set.FailedQueryIDs[id] = true
	}
	return set
}

func Merge(sets ...EvidenceSet) EvidenceSet {
	merged := EvidenceSet{
		EvidenceIDs:    map[string]bool{},
		FailedQueryIDs: map[string]bool{},
	}
	for _, set := range sets {
		for id := range set.EvidenceIDs {
			merged.EvidenceIDs[id] = true
		}
		for id := range set.FailedQueryIDs {
			merged.FailedQueryIDs[id] = true
		}
	}
	return merged
}

func ValidateAnswer(answer string, available EvidenceSet) ValidationResult {
	answerIDs := ExtractIDs(answer)
	var reasons []string
	for id := range answerIDs.EvidenceIDs {
		if !available.EvidenceIDs[id] {
			reasons = append(reasons, "answer cites unknown evidence_id "+id)
		}
	}
	for id := range answerIDs.FailedQueryIDs {
		if !available.FailedQueryIDs[id] {
			reasons = append(reasons, "answer cites unknown failed_query_id "+id)
		}
	}
	if len(available.FailedQueryIDs) > 0 && len(answerIDs.FailedQueryIDs) == 0 && !containsUnverifiedLanguage(answer) {
		reasons = append(reasons, "failed tool calls exist but answer does not mark any evidence gap")
	}
	if len(available.EvidenceIDs) > 0 && len(answerIDs.EvidenceIDs) == 0 && containsStrongDiagnosisLanguage(answer) {
		reasons = append(reasons, "answer makes diagnostic claims without citing evidence_id")
	}
	return ValidationResult{OK: len(reasons) == 0, Reasons: reasons}
}

func containsUnverifiedLanguage(answer string) bool {
	for _, marker := range []string{
		"未验证", "证据不足", "证据缺口", "无法确认", "无法判断", "查询失败", "不可用", "unknown", "not verified",
	} {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func containsStrongDiagnosisLanguage(answer string) bool {
	for _, marker := range []string{
		"判断", "结论", "根因", "原因", "导致", "确认", "已确认", "异常", "故障",
	} {
		if strings.Contains(answer, marker) {
			return true
		}
	}
	return false
}
