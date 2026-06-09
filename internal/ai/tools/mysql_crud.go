package tools

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MysqlCrudInput struct {
	DSN         string `json:"dsn,omitempty" jsonschema:"description=Deprecated. The server configured DSN is always used; user supplied DSN values are ignored"`
	SQL         string `json:"sql" jsonschema:"description=The SQL query to execute against the configured MySQL database"`
	OperateType string `json:"operate_type" jsonschema:"description=The SQL operation type: query, insert, update, or delete"`
}

func NewMysqlCrudTool() (tool.InvokableTool, error) {
	t, err := utils.InferOptionableTool(
		"mysql_crud",
		"Execute SQL queries against the server configured MySQL database and return results in JSON format. Use operate_type=query for reads. Write operations require server-side enablement.",
		func(ctx context.Context, input *MysqlCrudInput, opts ...tool.Option) (string, error) {
			if input == nil {
				return marshalToolError(fmt.Errorf("empty mysql input"), "invalid mysql input"), nil
			}
			input.OperateType = strings.ToLower(strings.TrimSpace(input.OperateType))
			if input.OperateType == "" {
				input.OperateType = "query"
			}
			input.SQL = strings.TrimSpace(input.SQL)
			if input.SQL == "" {
				return marshalToolError(fmt.Errorf("empty sql"), "invalid mysql input"), nil
			}
			if input.OperateType == "query" {
				guardedSQL, guardErr := guardReadOnlySQL(ctx, input.SQL)
				if guardErr != nil {
					return marshalToolError(guardErr, "mysql read query rejected by tool policy"), nil
				}
				input.SQL = guardedSQL
			}
			if IsWriteOperation(input.OperateType) && !mysqlWriteEnabled(ctx) {
				return marshalToolError(
					fmt.Errorf("mysql %s is disabled by tool policy", input.OperateType),
					"mysql write operations require explicit server configuration",
				), nil
			}

			dsn, err := configuredMysqlDSN(ctx)
			if err != nil {
				return marshalToolError(err, "load mysql configuration failed"), nil
			}
			db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
			if err != nil {
				return marshalToolError(err, "connect mysql failed"), nil
			}
			sqlDB, err := db.DB()
			if err == nil {
				defer sqlDB.Close()
			}

			switch input.OperateType {
			case "query":
				var results []map[string]any
				if err = db.WithContext(ctx).Raw(input.SQL).Scan(&results).Error; err != nil {
					return marshalToolError(err, "execute mysql query failed"), nil
				}
				return marshalToolData(results, fmt.Sprintf("query returned %d rows", len(results))), nil
			case "insert", "update", "delete":
				result := db.WithContext(ctx).Exec(input.SQL)
				if result.Error != nil {
					return marshalToolError(result.Error, "execute mysql statement failed"), nil
				}
				return marshalToolData(map[string]any{"rows_affected": result.RowsAffected}, "mysql statement executed"), nil
			default:
				return marshalToolError(fmt.Errorf("unsupported operate_type %q", input.OperateType), "invalid mysql operation"), nil
			}
		})
	if err != nil {
		return nil, err
	}
	return t, nil
}

var (
	mysqlLimitCommaPattern = regexp.MustCompile(`(?i)\blimit\s+([0-9]+)\s*,\s*([0-9]+)\b`)
	mysqlLimitCountPattern = regexp.MustCompile(`(?i)\blimit\s+([0-9]+)(\s+offset\s+[0-9]+)?\b`)
)

func guardReadOnlySQL(ctx context.Context, sql string) (string, error) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return "", fmt.Errorf("empty sql")
	}
	trimmed = strings.TrimSuffix(trimmed, ";")
	if strings.Contains(trimmed, ";") {
		return "", fmt.Errorf("multiple mysql statements are not allowed")
	}

	lower := strings.ToLower(strings.TrimSpace(trimmed))
	if strings.Contains(lower, "--") || strings.Contains(lower, "/*") || strings.Contains(lower, "*/") || strings.Contains(lower, "#") {
		return "", fmt.Errorf("sql comments are not allowed")
	}
	if hasForbiddenSQLKeyword(lower) {
		return "", fmt.Errorf("sql contains a forbidden keyword")
	}
	if !(strings.HasPrefix(lower, "select ") || strings.HasPrefix(lower, "show ") || strings.HasPrefix(lower, "explain ")) {
		return "", fmt.Errorf("only SELECT, SHOW, and EXPLAIN statements are allowed")
	}

	if strings.HasPrefix(lower, "select ") {
		return applySelectLimit(ctx, trimmed), nil
	}
	return trimmed, nil
}

func hasForbiddenSQLKeyword(lowerSQL string) bool {
	for _, keyword := range []string{
		"insert", "update", "delete", "drop", "alter", "create", "truncate", "replace",
		"grant", "revoke", "call", "set", "commit", "rollback", "start", "begin",
		"lock", "unlock", "load", "outfile", "dumpfile", "into outfile", "into dumpfile",
	} {
		if regexp.MustCompile(`\b`+regexp.QuoteMeta(keyword)+`\b`).FindStringIndex(lowerSQL) != nil {
			return true
		}
	}
	return false
}

func applySelectLimit(ctx context.Context, sql string) string {
	maxRows := mysqlMaxRows(ctx)
	if maxRows <= 0 {
		return sql
	}
	if matches := mysqlLimitCommaPattern.FindStringSubmatchIndex(sql); matches != nil {
		return clampSQLNumberAt(sql, matches[4], matches[5], maxRows)
	}
	if matches := mysqlLimitCountPattern.FindStringSubmatchIndex(sql); matches != nil {
		return clampSQLNumberAt(sql, matches[2], matches[3], maxRows)
	}
	return fmt.Sprintf("%s LIMIT %d", sql, maxRows)
}

func clampSQLNumberAt(sql string, start int, end int, maxRows int) string {
	limitText := sql[start:end]
	limit, err := strconv.Atoi(limitText)
	if err != nil || limit > maxRows {
		return sql[:start] + strconv.Itoa(maxRows) + sql[end:]
	}
	return sql
}

func mysqlMaxRows(ctx context.Context) int {
	v, err := g.Cfg().Get(ctx, "tools.mysql.max_rows")
	if err != nil || v == nil {
		return 100
	}
	maxRows := v.Int()
	if maxRows <= 0 {
		return 100
	}
	return maxRows
}

func mysqlWriteEnabled(ctx context.Context) bool {
	v, err := g.Cfg().Get(ctx, "tools.mysql.allow_write")
	if err != nil {
		return false
	}
	return v.Bool()
}

func configuredMysqlDSN(ctx context.Context) (string, error) {
	user, err := cfgString(ctx, "database.default.user")
	if err != nil {
		return "", err
	}
	pass, err := cfgString(ctx, "database.default.pass")
	if err != nil {
		return "", err
	}
	host, err := cfgString(ctx, "database.default.host")
	if err != nil {
		return "", err
	}
	port, err := cfgString(ctx, "database.default.port")
	if err != nil {
		return "", err
	}
	name, err := cfgString(ctx, "database.default.name")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, name), nil
}

func cfgString(ctx context.Context, key string) (string, error) {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(v.String())
	if s == "" {
		return "", fmt.Errorf("missing config %s", key)
	}
	return s, nil
}
