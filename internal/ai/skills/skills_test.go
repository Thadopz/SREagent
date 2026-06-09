package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseFileReadsFrontMatterAndBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	writeSkill(t, path, `---
name: order-diagnostics
description: Use this skill when diagnosing order timeout incidents.
---

# Order Diagnostics

Check order lifecycle evidence first.
`)

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("expected valid skill, got err=%v", err)
	}
	if got.Name != "order-diagnostics" {
		t.Fatalf("expected skill name, got %q", got.Name)
	}
	if !strings.Contains(got.Description, "order timeout") {
		t.Fatalf("expected description, got %q", got.Description)
	}
	if !strings.Contains(got.Body, "Check order lifecycle evidence first.") {
		t.Fatalf("expected body, got %q", got.Body)
	}
}

func TestParseFileRequiresFrontMatterNameAndDescription(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	writeSkill(t, path, `---
name: broken
---

# Broken
`)

	if _, err := ParseFile(path); err == nil {
		t.Fatal("expected missing description error")
	}
}

func TestSelectMatchesByMetadataAndLimitsBody(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "order", "SKILL.md")
	second := filepath.Join(dir, "payment", "SKILL.md")
	writeSkill(t, first, `---
name: order-diagnostics
description: Use this skill when diagnosing order timeout incidents.
---

`+strings.Repeat("a", 40))
	writeSkill(t, second, `---
name: payment-callback
description: Use this skill when diagnosing payment callback failures.
---

Payment callback steps.
`)

	got, err := Select(context.Background(), Config{
		Dirs:         []string{dir},
		MaxSelected:  1,
		MaxBodyChars: 10,
	}, "please diagnose this order timeout")
	if err != nil {
		t.Fatalf("expected select success, got err=%v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one selected skill, got %d", len(got))
	}
	if got[0].Name != "order-diagnostics" {
		t.Fatalf("expected order skill, got %q", got[0].Name)
	}
	if utf8.RuneCountInString(got[0].Body) != 10 {
		t.Fatalf("expected trimmed body to 10 runes, got %d", utf8.RuneCountInString(got[0].Body))
	}
}

func TestSelectMatchesChineseQueryWithoutSpaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "order", "SKILL.md")
	writeSkill(t, path, `---
name: order-diagnostics
description: Use this skill for 订单超时 troubleshooting.
---

Check order lifecycle evidence first.
`)

	got, err := Select(context.Background(), Config{
		Dirs:         []string{dir},
		MaxSelected:  1,
		MaxBodyChars: 100,
	}, "订单超时怎么办")
	if err != nil {
		t.Fatalf("expected select success, got err=%v", err)
	}
	if len(got) != 1 || got[0].Name != "order-diagnostics" {
		t.Fatalf("expected order skill, got %+v", got)
	}
}

func TestFormatSelectedWrapsSkillBlocks(t *testing.T) {
	got := FormatSelected([]Skill{{
		Name: "order-diagnostics",
		Body: "Check evidence.",
	}})
	if !strings.Contains(got, "==== Skill: order-diagnostics ====") {
		t.Fatalf("expected skill header, got %q", got)
	}
	if !strings.Contains(got, "==== End Skill ====") {
		t.Fatalf("expected skill footer, got %q", got)
	}
}

func writeSkill(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}
