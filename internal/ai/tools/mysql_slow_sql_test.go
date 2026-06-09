package tools

import (
	"strings"
	"testing"
)

func TestBuildMysqlSlowSQLQueryDefaultsToPerformanceSchema(t *testing.T) {
	query, err := buildMysqlSlowSQLQuery(nil)
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	if query.Source != "performance_schema" {
		t.Fatalf("expected performance_schema source, got %q", query.Source)
	}
	if query.SortBy != "avg_latency" {
		t.Fatalf("expected avg_latency sort, got %q", query.SortBy)
	}
	if query.Limit != defaultSlowSQLLimit {
		t.Fatalf("expected default limit %d, got %d", defaultSlowSQLLimit, query.Limit)
	}
	if !strings.Contains(query.SQL, "performance_schema.events_statements_summary_by_digest") {
		t.Fatalf("expected performance_schema query, got %s", query.SQL)
	}
	if !strings.Contains(query.SQL, "ORDER BY AVG_TIMER_WAIT DESC") {
		t.Fatalf("expected avg latency ordering, got %s", query.SQL)
	}
}

func TestBuildMysqlSlowSQLQueryClampsLimit(t *testing.T) {
	query, err := buildMysqlSlowSQLQuery(&MysqlSlowSQLInput{
		Source: "sys",
		SortBy: "rows_examined",
		Limit:  500,
	})
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	if query.Limit != maxSlowSQLLimit {
		t.Fatalf("expected max limit %d, got %d", maxSlowSQLLimit, query.Limit)
	}
	if !strings.Contains(query.SQL, "FROM sys.x$statement_analysis") {
		t.Fatalf("expected sys query, got %s", query.SQL)
	}
	if !strings.Contains(query.SQL, "ORDER BY rows_examined DESC") {
		t.Fatalf("expected rows_examined ordering, got %s", query.SQL)
	}
}

func TestBuildMysqlSlowSQLQueryRejectsUnsupportedSourceAndSort(t *testing.T) {
	if _, err := buildMysqlSlowSQLQuery(&MysqlSlowSQLInput{Source: "information_schema"}); err == nil {
		t.Fatal("expected unsupported source error")
	}
	if _, err := buildMysqlSlowSQLQuery(&MysqlSlowSQLInput{Source: "performance_schema", SortBy: "drop table"}); err == nil {
		t.Fatal("expected unsupported sort error")
	}
}

func TestBuildMysqlSlowSQLQuerySlowLog(t *testing.T) {
	query, err := buildMysqlSlowSQLQuery(&MysqlSlowSQLInput{
		Source: "slow_log",
		SortBy: "recent",
		Limit:  -10,
	})
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	if query.Limit != 1 {
		t.Fatalf("expected minimum limit 1, got %d", query.Limit)
	}
	if !strings.Contains(query.SQL, "FROM mysql.slow_log") {
		t.Fatalf("expected slow log query, got %s", query.SQL)
	}
	if !strings.Contains(query.SQL, "ORDER BY start_time DESC") {
		t.Fatalf("expected recent ordering, got %s", query.SQL)
	}
}
