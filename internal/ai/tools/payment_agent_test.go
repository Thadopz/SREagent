package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPaymentAgentURLUsesConfiguredBase(t *testing.T) {
	values := url.Values{}
	values.Set("status", "OPEN")
	values.Set("type", "PAYMENT_FAILURE_RATE_HIGH")
	u, err := paymentAgentURL(context.Background(), "/api/v1/agent/query_anomaly_event", values)
	if err != nil {
		t.Fatalf("build url: %v", err)
	}
	if !strings.HasPrefix(u, defaultPaymentAgentBaseURL+"/api/v1/agent/query_anomaly_event?") {
		t.Fatalf("unexpected url %q", u)
	}
	if !strings.Contains(u, "status=OPEN") || !strings.Contains(u, "type=PAYMENT_FAILURE_RATE_HIGH") {
		t.Fatalf("missing query params in %q", u)
	}
}

func TestQueryPaymentAnomalyToolCallsAgent(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "OK",
			"data": map[string]any{
				"events": []map[string]any{{
					"type":     "PAYMENT_FAILURE_RATE_HIGH",
					"severity": "CRITICAL",
				}},
			},
		})
	}))
	defer server.Close()

	oldBase := defaultPaymentAgentBaseURL
	defaultPaymentAgentBaseURL = server.URL
	defer func() { defaultPaymentAgentBaseURL = oldBase }()

	values := url.Values{}
	values.Set("status", "OPEN")
	values.Set("limit", "5")
	out, err := callPaymentAgent(context.Background(), "/api/v1/agent/query_anomaly_event", values, "ok")
	if err != nil {
		t.Fatalf("call payment agent: %v", err)
	}
	if gotPath != "/api/v1/agent/query_anomaly_event" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if !strings.Contains(gotQuery, "status=OPEN") || !strings.Contains(gotQuery, "limit=5") {
		t.Fatalf("unexpected query %q", gotQuery)
	}
	if !strings.Contains(out, "PAYMENT_FAILURE_RATE_HIGH") {
		t.Fatalf("expected anomaly response in output, got %s", out)
	}
}

func TestRequirePaymentOrderNo(t *testing.T) {
	if _, err := requirePaymentOrderNo(nil); err == nil {
		t.Fatal("expected nil input error")
	}
	if _, err := requirePaymentOrderNo(&PaymentOrderDetailInput{}); err == nil {
		t.Fatal("expected empty order error")
	}
	got, err := requirePaymentOrderNo(&PaymentOrderDetailInput{PaymentOrderNo: " P123 "})
	if err != nil {
		t.Fatalf("require order: %v", err)
	}
	if got != "P123" {
		t.Fatalf("expected trimmed order no, got %q", got)
	}
}
