package authz

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

type Permission string

const (
	PermDocsRead    Permission = "tool:docs.read"
	PermMetricsRead Permission = "tool:metrics.read"
	PermLogsRead    Permission = "tool:logs.read"
	PermMySQLRead   Permission = "tool:mysql.read"
	PermMySQLWrite  Permission = "tool:mysql.write"
	PermPaymentRead Permission = "tool:payment.read"
	PermTimeRead    Permission = "tool:time.read"
)

type RiskLevel string

const (
	RiskReadOnly RiskLevel = "read_only"
	RiskWrite    RiskLevel = "write_or_risky"
)

type Principal struct {
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
	TenantID string   `json:"tenant_id"`
}

type ToolCall struct {
	Name               string
	Permission         Permission
	Risk               RiskLevel
	RequiresConfirm    bool
	Confirmed          bool
	ConversationID     string
	ArgumentsInJSON    string
	Operation          string
	ServerWriteEnabled bool
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

type Authorizer struct{}

type contextKey struct{}

const (
	headerUserID   = "X-User-Id"
	headerRoles    = "X-User-Roles"
	headerTenantID = "X-Tenant-Id"
)

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, normalizePrincipal(ctx, principal))
}

func PrincipalFromContext(ctx context.Context) Principal {
	if principal, ok := ctx.Value(contextKey{}).(Principal); ok {
		return normalizePrincipal(ctx, principal)
	}
	return normalizePrincipal(ctx, Principal{})
}

func PrincipalFromRequest(ctx context.Context, req *ghttp.Request, fallbackUserID string) Principal {
	principal := Principal{}
	if req != nil {
		principal.UserID = strings.TrimSpace(req.GetHeader(headerUserID))
		principal.Roles = splitCSV(req.GetHeader(headerRoles))
		principal.TenantID = strings.TrimSpace(req.GetHeader(headerTenantID))
	}
	if principal.UserID == "" {
		principal.UserID = strings.TrimSpace(fallbackUserID)
	}
	return normalizePrincipal(ctx, principal)
}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

func (a *Authorizer) Authorize(ctx context.Context, call ToolCall) Decision {
	if !permissionsEnabled(ctx) {
		return Decision{Allowed: true, Reason: "permissions disabled"}
	}
	principal := PrincipalFromContext(ctx)
	if call.Permission == "" {
		return Decision{Allowed: false, Reason: fmt.Sprintf("tool %s has no permission mapping", call.Name)}
	}
	if !principal.HasPermission(ctx, call.Permission) {
		return Decision{Allowed: false, Reason: fmt.Sprintf("missing permission %s", call.Permission)}
	}
	if call.Risk == RiskWrite {
		if !call.ServerWriteEnabled {
			return Decision{Allowed: false, Reason: "server write policy disabled"}
		}
		if call.RequiresConfirm && !call.Confirmed {
			return Decision{Allowed: false, Reason: "write operation requires confirmation"}
		}
	}
	return Decision{Allowed: true, Reason: "allowed"}
}

func (p Principal) HasPermission(ctx context.Context, permission Permission) bool {
	permissions := PermissionsForRoles(ctx, p.Roles)
	return permissions[permission]
}

func PermissionsForRoles(ctx context.Context, roles []string) map[Permission]bool {
	roles = normalizeRoles(ctx, roles)
	out := make(map[Permission]bool)
	for _, role := range roles {
		for _, permission := range scopesForRole(ctx, role) {
			out[permission] = true
		}
	}
	return out
}

func normalizePrincipal(ctx context.Context, principal Principal) Principal {
	principal.UserID = strings.TrimSpace(principal.UserID)
	principal.TenantID = strings.TrimSpace(principal.TenantID)
	principal.Roles = normalizeRoles(ctx, principal.Roles)
	if principal.TenantID == "" {
		principal.TenantID = "default"
	}
	return principal
}

func normalizeRoles(ctx context.Context, roles []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(roles)+1)
	for _, role := range roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		normalized = append(normalized, role)
	}
	if len(normalized) == 0 {
		role := strings.ToLower(strings.TrimSpace(configString(ctx, "permissions.default_role", "viewer")))
		if role == "" {
			role = "viewer"
		}
		normalized = append(normalized, role)
	}
	sort.Strings(normalized)
	return normalized
}

func scopesForRole(ctx context.Context, role string) []Permission {
	configKey := fmt.Sprintf("permissions.roles.%s.scopes", role)
	v, err := g.Cfg().Get(ctx, configKey)
	if err == nil && v != nil {
		rawScopes := v.Strings()
		if len(rawScopes) > 0 {
			return parsePermissions(rawScopes)
		}
	}

	switch role {
	case "admin":
		return []Permission{PermDocsRead, PermTimeRead, PermMetricsRead, PermLogsRead, PermPaymentRead, PermMySQLRead, PermMySQLWrite}
	case "oncall":
		return []Permission{PermDocsRead, PermTimeRead, PermMetricsRead, PermLogsRead, PermPaymentRead, PermMySQLRead}
	default:
		return []Permission{PermDocsRead, PermTimeRead}
	}
}

func parsePermissions(scopes []string) []Permission {
	out := make([]Permission, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		out = append(out, Permission(scope))
	}
	return out
}

func permissionsEnabled(ctx context.Context) bool {
	v, err := g.Cfg().Get(ctx, "permissions.enabled")
	if err != nil || v == nil {
		return true
	}
	return v.Bool()
}

func configString(ctx context.Context, key string, fallback string) string {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil || v == nil {
		return fallback
	}
	if s := strings.TrimSpace(v.String()); s != "" {
		return s
	}
	return fallback
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
