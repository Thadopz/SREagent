package authz

import (
	"context"
	"testing"
)

func TestPermissionsForDefaultViewerRole(t *testing.T) {
	perms := PermissionsForRoles(context.Background(), nil)
	if !perms[PermDocsRead] || !perms[PermTimeRead] {
		t.Fatalf("expected default viewer docs/time permissions, got %#v", perms)
	}
	if perms[PermMySQLRead] || perms[PermPaymentRead] || perms[PermLogsRead] {
		t.Fatalf("expected default viewer to exclude operational scopes, got %#v", perms)
	}
}

func TestPermissionsForOncallAndAdminRoles(t *testing.T) {
	ctx := context.Background()
	oncall := PermissionsForRoles(ctx, []string{"oncall"})
	if !oncall[PermMetricsRead] || !oncall[PermLogsRead] || !oncall[PermMySQLRead] || !oncall[PermPaymentRead] {
		t.Fatalf("expected oncall operational read scopes, got %#v", oncall)
	}
	if oncall[PermMySQLWrite] {
		t.Fatalf("expected oncall to exclude mysql write, got %#v", oncall)
	}

	admin := PermissionsForRoles(ctx, []string{"admin"})
	if !admin[PermMySQLWrite] {
		t.Fatalf("expected admin mysql write scope, got %#v", admin)
	}
}

func TestPrincipalFromRequestFallsBackToUserIDAndViewer(t *testing.T) {
	principal := PrincipalFromRequest(context.Background(), nil, "fallback-user")
	if principal.UserID != "fallback-user" {
		t.Fatalf("expected fallback user id, got %q", principal.UserID)
	}
	if len(principal.Roles) != 1 || principal.Roles[0] != "viewer" {
		t.Fatalf("expected default viewer role, got %#v", principal.Roles)
	}
	if principal.TenantID != "default" {
		t.Fatalf("expected default tenant, got %q", principal.TenantID)
	}
}

func TestAuthorizerDeniesAdminWriteWhenServerPolicyDisabled(t *testing.T) {
	ctx := WithPrincipal(context.Background(), Principal{UserID: "admin", Roles: []string{"admin"}})
	decision := NewAuthorizer().Authorize(ctx, ToolCall{
		Name:               "mysql_crud",
		Permission:         PermMySQLWrite,
		Risk:               RiskWrite,
		RequiresConfirm:    true,
		ServerWriteEnabled: false,
	})
	if decision.Allowed {
		t.Fatalf("expected mysql write to be denied when server write policy is disabled")
	}
}
