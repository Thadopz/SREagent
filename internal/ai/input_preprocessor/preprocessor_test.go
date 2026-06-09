package input_preprocessor

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var fixedNow = time.Date(2026, time.June, 8, 21, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))

func TestCleanQueryRemovesControlsAndTruncates(t *testing.T) {
	got := CleanQuery(" \u0000支付\n\t 回调\u0007 失败 ", 20)
	if got != "支付 回调 失败" {
		t.Fatalf("expected cleaned query, got %q", got)
	}
	long := CleanQuery(strings.Repeat("界", 30), 7)
	if utf8.RuneCountInString(long) != 7 {
		t.Fatalf("expected 7 runes, got %d", utf8.RuneCountInString(long))
	}
}

func TestProcessRecognizesPaymentDiagnosis(t *testing.T) {
	got := Process(context.Background(), "支付回调失败，帮我看下", fixedNow)
	if got.Intent != IntentDiagnose || got.Domain != DomainPayment {
		t.Fatalf("expected payment diagnosis, got intent=%s domain=%s", got.Intent, got.Domain)
	}
	if got.TimeRange == nil || got.TimeRange.Source != "default" {
		t.Fatalf("expected default time range, got %#v", got.TimeRange)
	}
	want := []string{"query_payment_anomaly", "query_payment_metrics", "query_payment_status_distribution"}
	if !toolNamesEqual(got.ToolHints, want) {
		t.Fatalf("expected tools %v, got %#v", want, got.ToolHints)
	}
}

func TestProcessRecognizesOrderQueryAndEntities(t *testing.T) {
	got := Process(context.Background(), "查一下订单号 PO202606080001 trace_id=tr-abc error_code=ERR_PAY_TIMEOUT", fixedNow)
	if got.Intent != IntentQuery || got.Domain != DomainPayment {
		t.Fatalf("expected payment query, got intent=%s domain=%s", got.Intent, got.Domain)
	}
	if got.Entities["payment_order_no"] != "PO202606080001" {
		t.Fatalf("expected payment order no, got %#v", got.Entities)
	}
	if got.Entities["trace_id"] != "tr-abc" || got.Entities["error_code"] != "ERR_PAY_TIMEOUT" {
		t.Fatalf("expected trace and error code, got %#v", got.Entities)
	}
	want := []string{"query_payment_anomaly", "query_payment_metrics", "query_payment_status_distribution", "query_payment_order_detail", "check_payment_consistency", "search_payment_logs"}
	if !toolNamesEqual(got.ToolHints, want) {
		t.Fatalf("expected tools %v, got %#v", want, got.ToolHints)
	}
}

func TestProcessRecognizesSlowSQLAlertDocsAndChat(t *testing.T) {
	cases := []struct {
		query  string
		intent string
		domain string
	}{
		{"最近慢 SQL 是什么", IntentQuery, DomainSlowSQL},
		{"这个告警怎么处理，按手册来", IntentPlan, DomainAlert},
		{"查一下内部文档 runbook", IntentQuery, DomainKnowledge},
		{"hello", IntentChat, DomainGeneral},
	}
	for _, tt := range cases {
		t.Run(tt.query, func(t *testing.T) {
			got := Process(context.Background(), tt.query, fixedNow)
			if got.Intent != tt.intent || got.Domain != tt.domain {
				t.Fatalf("expected %s/%s, got %s/%s", tt.intent, tt.domain, got.Intent, got.Domain)
			}
		})
	}
}

func TestCompleteTimeRange(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		intent    string
		wantStart time.Time
		wantEnd   time.Time
		wantNil   bool
	}{
		{name: "today", query: "今天支付失败", intent: IntentDiagnose, wantStart: time.Date(2026, 6, 8, 0, 0, 0, 0, fixedNow.Location()), wantEnd: fixedNow},
		{name: "yesterday", query: "昨天告警", intent: IntentQuery, wantStart: time.Date(2026, 6, 7, 0, 0, 0, 0, fixedNow.Location()), wantEnd: time.Date(2026, 6, 7, 23, 59, 59, 0, fixedNow.Location())},
		{name: "recent 30 minutes", query: "最近30分钟失败", intent: IntentDiagnose, wantStart: fixedNow.Add(-30 * time.Minute), wantEnd: fixedNow},
		{name: "recent 2 hours", query: "最近2小时慢SQL", intent: IntentQuery, wantStart: fixedNow.Add(-2 * time.Hour), wantEnd: fixedNow},
		{name: "default diagnosis", query: "支付失败", intent: IntentDiagnose, wantStart: fixedNow.Add(-30 * time.Minute), wantEnd: fixedNow},
		{name: "chat no time", query: "hello", intent: IntentChat, wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompleteTimeRange(tt.query, tt.intent, fixedNow)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil time range, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected time range")
			}
			if !got.Start.Equal(tt.wantStart) || !got.End.Equal(tt.wantEnd) {
				t.Fatalf("expected %v-%v, got %v-%v", tt.wantStart, tt.wantEnd, got.Start, got.End)
			}
		})
	}
}

func TestRiskAndMissingSlots(t *testing.T) {
	risk := Process(context.Background(), "帮我退款并重试这个订单", fixedNow)
	if risk.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk, got %s", risk.RiskLevel)
	}
	if len(risk.MissingSlots) != 1 || risk.MissingSlots[0] != "payment_order_no" {
		t.Fatalf("expected missing payment_order_no, got %#v", risk.MissingSlots)
	}

	global := Process(context.Background(), "支付是否异常", fixedNow)
	if len(global.MissingSlots) != 0 {
		t.Fatalf("expected no missing slots for global diagnosis, got %#v", global.MissingSlots)
	}
}

func TestFormatPromptIsStable(t *testing.T) {
	got := Process(context.Background(), "查订单 PO202606080001", fixedNow).FormatPrompt()
	for _, want := range []string{"Intent: query", "Domain: payment", "Entities: payment_order_no=PO202606080001", "Suggested Tools:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, got)
		}
	}
}

func toolNamesEqual(hints []ToolHint, want []string) bool {
	if len(hints) != len(want) {
		return false
	}
	for i := range want {
		if hints[i].Name != want[i] {
			return false
		}
	}
	return true
}
