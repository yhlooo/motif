package cli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/i18n"
)

func TestInstallResultGroupsPathsAndPreservesRuntimeStatus(t *testing.T) {
	root := t.TempDir()
	for _, locale := range []string{"en", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			ctx := i18n.ContextWithLocalizer(context.Background(), i18n.NewLocalizer(locale))
			result := installResult{
				root: root, changed: true, agents: []string{"codex", "claude", "codebuddy", "codebuddy"},
				manifest: integration.Manifest{
					Claude:    &integration.JSONHookConfig{File: integration.File{Path: filepath.Join(root, ".claude", "settings.local.json")}},
					Config:    integration.File{Path: filepath.Join(root, ".codex", "config.toml")},
					CodeBuddy: &integration.JSONHookConfig{File: integration.File{Path: filepath.Join(root, ".codebuddy", "settings.local.json")}},
					Hooks:     []integration.File{{Path: filepath.Join(root, ".git", "hooks", "post-commit")}, {Path: filepath.Join(root, ".git", "hooks", "post-checkout")}},
				},
				diagnostics: map[string]integration.Diagnostic{"codex": {Status: "active"}, "claude": {Status: "configured"}, "codebuddy": {Status: "configured"}},
			}
			var output bytes.Buffer
			if err := writeInstallResult(ctx, &output, result, false); err != nil {
				t.Fatal(err)
			}
			s := output.String()
			var colored bytes.Buffer
			if err := writeInstallResult(ctx, &colored, result, true); err != nil {
				t.Fatal(err)
			}
			if text := colored.String(); strings.Contains(text, "\x1b[33m") || strings.Contains(text, "\x1b[31m") || !strings.Contains(text, "    · CodeBuddy") || !strings.Contains(text, "    · Claude Code") || !strings.Contains(text, "CodeBuddy   \x1b[2m") || !strings.Contains(text, "Claude Code \x1b[2m") {
				t.Fatal(text)
			}

			for _, want := range []string{"    ✓ Codex\n      .codex/config.toml", "    ✓ CodeBuddy\n      .codebuddy/settings.local.json", "    ✓ post-commit", "    ✓ post-checkout", "    · CodeBuddy", "    · Claude Code"} {
				if !strings.Contains(s, want) {
					t.Fatalf("missing %q:\n%s", want, s)
				}
			}
			if strings.Contains(s, "\x1b[") || strings.Contains(s, root) || strings.Count(s, ".git/hooks/") != 1 || strings.Count(s, "✓ CodeBuddy") != 1 {
				t.Fatal(s)
			}
			if locale == "zh-CN" {
				for _, want := range []string{"安装完成", "Agent 配置", "生效状态", "已加载，允许执行", "配置已写入，重启客户端后生效"} {
					if !strings.Contains(s, want) {
						t.Fatal(s)
					}
				}
			} else if !strings.Contains(s, "Installation complete") || !strings.Contains(s, "restart the client to apply") {
				t.Fatal(s)
			}
		})
	}
}

func TestInstallResultShowsOnlyNewBackupsAndKeepsConflictDetails(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, ".git", "motif", "original-hooks", "post-commit")
	result := installResult{
		root: root, agents: []string{"codex"},
		manifest:    integration.Manifest{Config: integration.File{Path: filepath.Join(root, ".codex", "config.toml")}, Hooks: []integration.File{{Path: filepath.Join(root, ".git", "hooks", "post-commit"), Backup: backup}}},
		diagnostics: map[string]integration.Diagnostic{"codex": {Status: "conflict", Detail: "changed hook\nkeep user edits"}},
	}
	ctx := i18n.ContextWithLocalizer(context.Background(), i18n.NewLocalizer("en"))
	var output bytes.Buffer
	if err := writeInstallResult(ctx, &output, result, true); err != nil {
		t.Fatal(err)
	}
	if s := output.String(); strings.Contains(s, "Original hook backups") || !strings.Contains(s, "\x1b[31m✗\x1b[0m") || !strings.Contains(s, "      keep user edits") || !strings.Contains(s, "Already configured") {
		t.Fatal(s)
	}
	result.changes = []integration.FileChange{{Path: backup, Action: "create"}}
	result.hooksPlan = integration.HooksPlan{Override: true, Inherited: "/global/hooks", Path: filepath.Join(root, ".git", "hooks")}
	output.Reset()
	if err := writeInstallResult(ctx, &output, result, false); err != nil {
		t.Fatal(err)
	}
	if s := output.String(); !strings.Contains(s, "Original hook backups") || !strings.Contains(s, ".git/motif/original-hooks/post-commit") || !strings.Contains(s, "/global/hooks → .git/hooks") {
		t.Fatal(s)
	}
	if installColor(io.Discard) {
		t.Fatal("color enabled for redirected output")
	}
}
