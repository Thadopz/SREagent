package tools

import "SREagent/internal/authz"

type RiskLevel = authz.RiskLevel

const (
	RiskReadOnly RiskLevel = authz.RiskReadOnly
	RiskWrite    RiskLevel = authz.RiskWrite
)

type Metadata struct {
	Name            string
	Risk            RiskLevel
	Permission      authz.Permission
	RequiresConfirm bool
	Description     string
}

var metadataByName = map[string]Metadata{
	"query_internal_docs": {
		Name:        "query_internal_docs",
		Risk:        RiskReadOnly,
		Permission:  authz.PermDocsRead,
		Description: "Search internal documents and knowledge base.",
	},
	"get_current_time": {
		Name:        "get_current_time",
		Risk:        RiskReadOnly,
		Permission:  authz.PermTimeRead,
		Description: "Return the current time.",
	},
	"query_prometheus_alerts": {
		Name:        "query_prometheus_alerts",
		Risk:        RiskReadOnly,
		Permission:  authz.PermMetricsRead,
		Description: "Query alert and metrics data.",
	},
	"mysql_crud": {
		Name:            "mysql_crud",
		Risk:            RiskWrite,
		Permission:      authz.PermMySQLRead,
		RequiresConfirm: true,
		Description:     "Run MySQL statements. Write operations are disabled unless explicitly enabled by server config.",
	},
	"mysql_slow_sql": {
		Name:        "mysql_slow_sql",
		Risk:        RiskReadOnly,
		Permission:  authz.PermMySQLRead,
		Description: "Find slow MySQL statements using fixed read-only diagnostic queries.",
	},
	"query_payment_anomaly": {
		Name:        "query_payment_anomaly",
		Risk:        RiskReadOnly,
		Permission:  authz.PermPaymentRead,
		Description: "Query payment anomaly events from the local payment observability agent.",
	},
	"query_payment_metrics": {
		Name:        "query_payment_metrics",
		Risk:        RiskReadOnly,
		Permission:  authz.PermPaymentRead,
		Description: "Query payment business metrics and success rates.",
	},
	"query_payment_order_detail": {
		Name:        "query_payment_order_detail",
		Risk:        RiskReadOnly,
		Permission:  authz.PermPaymentRead,
		Description: "Query payment order detail, transaction, refunds, and notify tasks.",
	},
	"check_payment_consistency": {
		Name:        "check_payment_consistency",
		Risk:        RiskReadOnly,
		Permission:  authz.PermPaymentRead,
		Description: "Check payment order and transaction consistency.",
	},
	"search_payment_logs": {
		Name:        "search_payment_logs",
		Risk:        RiskReadOnly,
		Permission:  authz.PermPaymentRead,
		Description: "Search payment diagnostic logs.",
	},
	"query_payment_status_distribution": {
		Name:        "query_payment_status_distribution",
		Risk:        RiskReadOnly,
		Permission:  authz.PermPaymentRead,
		Description: "Query recent payment order status distribution.",
	},
}

func LookupMetadata(name string) (Metadata, bool) {
	meta, ok := metadataByName[name]
	return meta, ok
}

func IsWriteOperation(operateType string) bool {
	switch operateType {
	case "insert", "update", "delete":
		return true
	default:
		return false
	}
}
