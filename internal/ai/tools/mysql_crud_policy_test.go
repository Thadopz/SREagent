package tools

import (
	"context"
	"strings"
	"testing"
)

func TestGuardReadOnlySQLRejectsUnsafeStatements(t *testing.T) {
	ctx := context.Background()
	for _, sql := range []string{
		"update orders set status='done'",
		"delete from orders",
		"drop table orders",
		"select * from orders; select * from users",
		"call rebuild_orders()",
		"begin",
	} {
		t.Run(sql, func(t *testing.T) {
			if _, err := guardReadOnlySQL(ctx, sql); err == nil {
				t.Fatalf("expected sql to be rejected: %s", sql)
			}
		})
	}
}

func TestGuardReadOnlySQLAllowsSelectShowExplain(t *testing.T) {
	ctx := context.Background()
	for _, sql := range []string{
		"select * from orders",
		"show tables",
		"explain select * from orders",
	} {
		t.Run(sql, func(t *testing.T) {
			if _, err := guardReadOnlySQL(ctx, sql); err != nil {
				t.Fatalf("expected sql to be allowed: %s err=%v", sql, err)
			}
		})
	}
}

func TestGuardReadOnlySQLAppliesDefaultLimit(t *testing.T) {
	got, err := guardReadOnlySQL(context.Background(), "select * from orders")
	if err != nil {
		t.Fatalf("guard read sql: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "limit 100") {
		t.Fatalf("expected default limit, got %q", got)
	}
}

func TestGuardReadOnlySQLPreservesSmallerLimit(t *testing.T) {
	got, err := guardReadOnlySQL(context.Background(), "select * from orders limit 10")
	if err != nil {
		t.Fatalf("guard read sql: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "limit 10") {
		t.Fatalf("expected smaller limit to be preserved, got %q", got)
	}
}

func TestGuardReadOnlySQLClampsLargerLimit(t *testing.T) {
	got, err := guardReadOnlySQL(context.Background(), "select * from orders limit 1000")
	if err != nil {
		t.Fatalf("guard read sql: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "limit 100") {
		t.Fatalf("expected larger limit to be clamped, got %q", got)
	}
}

func TestGuardReadOnlySQLClampsLimitWithOffsetKeyword(t *testing.T) {
	got, err := guardReadOnlySQL(context.Background(), "select * from orders limit 1000 offset 10")
	if err != nil {
		t.Fatalf("guard read sql: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "limit 100 offset 10") {
		t.Fatalf("expected row count to be clamped, got %q", got)
	}
}

func TestGuardReadOnlySQLClampsLimitWithCommaOffset(t *testing.T) {
	got, err := guardReadOnlySQL(context.Background(), "select * from orders limit 10, 1000")
	if err != nil {
		t.Fatalf("guard read sql: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "limit 10, 100") {
		t.Fatalf("expected comma row count to be clamped, got %q", got)
	}
}
