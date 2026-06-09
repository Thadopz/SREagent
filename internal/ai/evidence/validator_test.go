package evidence

import "testing"

func TestValidateAnswerRejectsUnknownEvidenceID(t *testing.T) {
	available := EvidenceSet{
		EvidenceIDs:    map[string]bool{"ev_1_abcd1234": true},
		FailedQueryIDs: map[string]bool{},
	}
	got := ValidateAnswer("confirmed by ev_2_abcd1234", available)
	if got.OK {
		t.Fatalf("expected unknown evidence id rejection")
	}
}

func TestValidateAnswerRequiresGapForFailedQuery(t *testing.T) {
	available := EvidenceSet{
		EvidenceIDs:    map[string]bool{},
		FailedQueryIDs: map[string]bool{"fq_1_abcd1234": true},
	}
	got := ValidateAnswer("the metric is normal", available)
	if got.OK {
		t.Fatalf("expected failed query gap rejection")
	}
	got = ValidateAnswer("metric 未验证 because fq_1_abcd1234 failed", available)
	if !got.OK {
		t.Fatalf("expected evidence gap answer to pass, got %#v", got)
	}
}

func TestValidateAnswerRequiresEvidenceForStrongDiagnosis(t *testing.T) {
	available := EvidenceSet{
		EvidenceIDs:    map[string]bool{"ev_1_abcd1234": true},
		FailedQueryIDs: map[string]bool{},
	}
	got := ValidateAnswer("判断：Redis 延迟异常导致支付失败", available)
	if got.OK {
		t.Fatalf("expected strong diagnosis without evidence id to fail")
	}
	got = ValidateAnswer("判断：Redis 延迟异常 ev_1_abcd1234", available)
	if !got.OK {
		t.Fatalf("expected cited diagnosis to pass, got %#v", got)
	}
}
