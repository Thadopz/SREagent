package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/gogf/gf/v2/frame/g"
)

var defaultPaymentAgentBaseURL = "http://127.0.0.1:8000"

type QueryPaymentAnomalyInput struct {
	Type           string `json:"type,omitempty" jsonschema:"description=Anomaly type filter, such as PAYMENT_FAILURE_RATE_HIGH, ORDER_SUCCESS_RATE_LOW, PAYMENT_STATE_MISMATCH, ORDER_PENDING_TOO_LONG"`
	Status         string `json:"status,omitempty" jsonschema:"description=Anomaly status filter, usually OPEN"`
	Severity       string `json:"severity,omitempty" jsonschema:"description=Severity filter, such as WARNING or CRITICAL"`
	RelatedOrderID string `json:"related_order_id,omitempty" jsonschema:"description=Payment order number to filter anomalies for a specific order"`
	Limit          int    `json:"limit,omitempty" jsonschema:"description=Maximum events to return; default is server-side default"`
}

type QueryPaymentMetricsInput struct {
	Service    string `json:"service,omitempty" jsonschema:"description=Service name, defaults to payment"`
	MetricName string `json:"metric_name,omitempty" jsonschema:"description=Metric name such as order_create_request, order_create_success, payment_success, payment_failed"`
	StartTime  string `json:"start_time,omitempty" jsonschema:"description=Start time, accepted formats include YYYY-MM-DD HH:mm:ss, YYYY-MM-DDTHH:mm:ss, or RFC3339"`
	EndTime    string `json:"end_time,omitempty" jsonschema:"description=End time, accepted formats include YYYY-MM-DD HH:mm:ss, YYYY-MM-DDTHH:mm:ss, or RFC3339"`
}

type PaymentOrderDetailInput struct {
	PaymentOrderNo string `json:"payment_order_no" jsonschema:"description=Payment order number to inspect"`
}

type SearchPaymentLogsInput struct {
	OrderID string `json:"order_id,omitempty" jsonschema:"description=Payment order number to filter diagnostic logs"`
	Level   string `json:"level,omitempty" jsonschema:"description=Log level such as INFO, WARN, or ERROR. Empty defaults to recent ERROR logs when no other filter is provided"`
	Keyword string `json:"keyword,omitempty" jsonschema:"description=Keyword to search in log message or error message"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum logs to return"`
}

type QueryPaymentStatusDistributionInput struct {
	Minutes int `json:"minutes,omitempty" jsonschema:"description=Recent time window in minutes; defaults to server-side default"`
}

func NewQueryPaymentAnomalyTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_payment_anomaly",
		"Query payment/order anomaly events from the local payment observability agent. Use this when the user asks whether payment is abnormal, whether there are open incidents, or needs evidence for business anomalies.",
		func(ctx context.Context, input *QueryPaymentAnomalyInput, opts ...tool.Option) (string, error) {
			values := url.Values{}
			if input != nil {
				addQuery(values, "type", input.Type)
				addQuery(values, "status", input.Status)
				addQuery(values, "severity", input.Severity)
				addQuery(values, "relatedOrderId", input.RelatedOrderID)
				addPositiveIntQuery(values, "limit", input.Limit)
			}
			return callPaymentAgent(ctx, "/api/v1/agent/query_anomaly_event", values, "payment anomaly query completed")
		})
}

func NewQueryPaymentMetricsTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_payment_metrics",
		"Query payment business metrics from the local payment observability agent, including order creation and payment success/failure rates.",
		func(ctx context.Context, input *QueryPaymentMetricsInput, opts ...tool.Option) (string, error) {
			values := url.Values{}
			service := "payment"
			if input != nil {
				if strings.TrimSpace(input.Service) != "" {
					service = input.Service
				}
				addQuery(values, "metricName", input.MetricName)
				addQuery(values, "startTime", input.StartTime)
				addQuery(values, "endTime", input.EndTime)
			}
			addQuery(values, "service", service)
			return callPaymentAgent(ctx, "/api/v1/agent/query_business_metric", values, "payment metrics query completed")
		})
}

func NewQueryPaymentOrderDetailTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_payment_order_detail",
		"Query payment order, latest transaction, refunds, and notify tasks from the local payment observability agent. Use this for order-level diagnosis.",
		func(ctx context.Context, input *PaymentOrderDetailInput, opts ...tool.Option) (string, error) {
			orderNo, err := requirePaymentOrderNo(input)
			if err != nil {
				return marshalToolError(err, "invalid payment order input"), nil
			}
			values := url.Values{}
			addQuery(values, "paymentOrderNo", orderNo)
			return callPaymentAgent(ctx, "/api/v1/agent/query_order_detail", values, "payment order detail query completed")
		})
}

func NewCheckPaymentConsistencyTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"check_payment_consistency",
		"Check whether a payment order and its latest transaction are internally consistent. Use this for state mismatch diagnosis.",
		func(ctx context.Context, input *PaymentOrderDetailInput, opts ...tool.Option) (string, error) {
			orderNo, err := requirePaymentOrderNo(input)
			if err != nil {
				return marshalToolError(err, "invalid payment order input"), nil
			}
			values := url.Values{}
			addQuery(values, "paymentOrderNo", orderNo)
			return callPaymentAgent(ctx, "/api/v1/agent/check_order_consistency", values, "payment consistency check completed")
		})
}

func NewSearchPaymentLogsTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"search_payment_logs",
		"Search structured diagnostic logs from the local payment observability agent. Use this to find payment.create, payment.callback, and error logs for an order or incident.",
		func(ctx context.Context, input *SearchPaymentLogsInput, opts ...tool.Option) (string, error) {
			values := url.Values{}
			if input != nil {
				addQuery(values, "orderId", input.OrderID)
				addQuery(values, "level", input.Level)
				addQuery(values, "keyword", input.Keyword)
				addPositiveIntQuery(values, "limit", input.Limit)
			}
			return callPaymentAgent(ctx, "/api/v1/agent/search_error_logs", values, "payment diagnostic logs query completed")
		})
}

func NewQueryPaymentStatusDistributionTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_payment_status_distribution",
		"Query recent payment order status distribution from the local payment observability agent. Use this to estimate current impact scope.",
		func(ctx context.Context, input *QueryPaymentStatusDistributionInput, opts ...tool.Option) (string, error) {
			values := url.Values{}
			if input != nil {
				addPositiveIntQuery(values, "minutes", input.Minutes)
			}
			return callPaymentAgent(ctx, "/api/v1/agent/query_recent_status_distribution", values, "payment status distribution query completed")
		})
}

func callPaymentAgent(ctx context.Context, path string, values url.Values, message string) (string, error) {
	endpoint, err := paymentAgentURL(ctx, path, values)
	if err != nil {
		return marshalToolError(err, "build payment agent request failed"), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return marshalToolError(err, "build payment agent request failed"), nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return marshalToolError(err, "call payment agent failed"), nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return marshalToolError(err, "read payment agent response failed"), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return marshalToolError(fmt.Errorf("payment agent http %d: %s", resp.StatusCode, string(body)), "payment agent returned non-success status"), nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return marshalToolError(err, "parse payment agent response failed"), nil
	}
	return marshalToolData(payload, message), nil
}

func paymentAgentURL(ctx context.Context, path string, values url.Values) (string, error) {
	base := paymentAgentBaseURL(ctx)
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = values.Encode()
	return u.String(), nil
}

func paymentAgentBaseURL(ctx context.Context) string {
	for _, key := range []string{"tools.payment_agent.base_url", "payment_agent.base_url"} {
		v, err := g.Cfg().Get(ctx, key)
		if err != nil || v == nil {
			continue
		}
		base := strings.TrimSpace(v.String())
		if base != "" {
			return strings.TrimRight(base, "/")
		}
	}
	return defaultPaymentAgentBaseURL
}

func addQuery(values url.Values, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values.Set(key, value)
	}
}

func addPositiveIntQuery(values url.Values, key string, value int) {
	if value > 0 {
		values.Set(key, strconv.Itoa(value))
	}
}

func requirePaymentOrderNo(input *PaymentOrderDetailInput) (string, error) {
	if input == nil {
		return "", fmt.Errorf("missing payment order input")
	}
	orderNo := strings.TrimSpace(input.PaymentOrderNo)
	if orderNo == "" {
		return "", fmt.Errorf("missing payment_order_no")
	}
	return orderNo, nil
}
