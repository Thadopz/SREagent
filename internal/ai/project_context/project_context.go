package project_context

import (
	"context"
	"errors"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	DefaultPath     = "AGENT.md"
	DefaultMaxChars = 12000
)

type Config struct {
	Enabled  bool
	Path     string
	MaxChars int
}

func LoadConfig(ctx context.Context) Config {
	return Config{
		Enabled:  configBool(ctx, "project_context.enabled", true),
		Path:     configString(ctx, "project_context.path", DefaultPath),
		MaxChars: configInt(ctx, "project_context.max_chars", DefaultMaxChars),
	}
}

func Load(ctx context.Context) string {
	content, err := LoadWithConfig(ctx, LoadConfig(ctx))
	if err != nil {
		g.Log().Warningf(ctx, "load project context failed, err=%v", err)
		return ""
	}
	return content
}

func LoadWithConfig(ctx context.Context, cfg Config) (string, error) {
	if !cfg.Enabled {
		return "", nil
	}
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = DefaultPath
	}
	maxChars := cfg.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	content := strings.TrimSpace(string(raw))
	if content == "" {
		return "", nil
	}
	if utf8.RuneCountInString(content) > maxChars {
		g.Log().Warningf(ctx, "project context %s exceeds max_chars=%d, truncating", path, maxChars)
		content = trimRunesTail(content, maxChars)
	}
	return content, nil
}

func trimRunesTail(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func configString(ctx context.Context, key string, fallback string) string {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil || v == nil {
		return fallback
	}
	out := strings.TrimSpace(v.String())
	if out == "" {
		return fallback
	}
	return out
}

func configBool(ctx context.Context, key string, fallback bool) bool {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil || v == nil {
		return fallback
	}
	return v.Bool()
}

func configInt(ctx context.Context, key string, fallback int) int {
	v, err := g.Cfg().Get(ctx, key)
	if err != nil || v == nil {
		return fallback
	}
	out := v.Int()
	if out <= 0 {
		return fallback
	}
	return out
}
