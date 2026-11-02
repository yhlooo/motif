package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/i18n"
)

func TestCodexHookStatusMessagesFollowInstallLanguage(t *testing.T) {
	for _, locale := range []string{"zh-CN", "en", "fr"} {
		t.Run(locale, func(t *testing.T) {
			r, _ := installProject(t)
			ctx := i18n.ContextWithLocalizer(context.Background(), i18n.NewLocalizer(locale))
			m, _, err := integration.Install(ctx, r, "/motif")
			if err != nil {
				t.Fatal(err)
			}
			en := map[string]string{
				"PreToolUse": "Motif snapshot before tool execution", "PostToolUse": "Motif snapshot after tool execution",
				"Stop": "Motif snapshot at turn end", "Interrupt": "Motif snapshot on interruption", "SessionEnd": "Motif snapshot at session end",
			}
			zh := map[string]string{
				"PreToolUse": "Motif 工具运行前快照", "PostToolUse": "Motif 工具运行后快照", "Stop": "Motif 回合结束快照",
				"Interrupt": "Motif 中断快照", "SessionEnd": "Motif 会话结束快照",
			}
			expected := en
			if locale == "zh-CN" {
				expected = zh
			}
			assertHookStatuses(t, m.Config.Content, expected)
			if _, changed, err := integration.Install(ctx, r, "/motif"); err != nil || changed {
				t.Fatal("localized install not idempotent", err)
			}
		})
	}
}

func assertHookStatuses(t *testing.T, content string, expected map[string]string) {
	t.Helper()
	var config struct {
		Hooks map[string][]struct {
			Hooks []struct {
				StatusMessage string `toml:"statusMessage"`
			} `toml:"hooks"`
		} `toml:"hooks"`
	}
	if err := toml.Unmarshal([]byte(content), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Hooks) != len(expected) {
		t.Fatal("unexpected hook count")
	}
	for event, text := range expected {
		groups := config.Hooks[event]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 || groups[0].Hooks[0].StatusMessage != text {
			t.Fatalf("incorrect status for %s: %+v", event, groups)
		}
	}
}

func TestInstallUpgradesOldHookLabelsAndCanChangeLanguage(t *testing.T) {
	r, _ := installProject(t)
	en := i18n.ContextWithLocalizer(context.Background(), i18n.NewLocalizer("en"))
	zh := i18n.ContextWithLocalizer(context.Background(), i18n.NewLocalizer("zh-CN"))
	m, _, err := integration.Install(en, r, "/motif")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(m.Config.Content, "\n")
	legacy := []string{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "statusMessage = ") {
			legacy = append(legacy, line)
		}
	}
	m.Config.Content = strings.Join(legacy, "\n")
	const userConfig = "\n# User configuration must survive an upgrade.\nmodel = \"user-model\"\n"
	if err = os.WriteFile(m.Config.Path, []byte(userConfig+m.Config.Content), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	if err = os.WriteFile(filepath.Join(r.GitDir, "motif", "install.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	upgraded, changed, err := integration.Install(zh, r, "/motif")
	if err != nil || !changed {
		t.Fatal("legacy upgrade", err)
	}
	if !strings.Contains(upgraded.Config.Content, `statusMessage = "Motif 工具运行前快照"`) {
		t.Fatal("missing localized label")
	}
	if err = integration.Verify(upgraded); err != nil {
		t.Fatal(err)
	}
	translated, changed, err := integration.Install(en, r, "/motif")
	if err != nil || !changed {
		t.Fatal("language switch", err)
	}
	if !strings.Contains(translated.Config.Content, `statusMessage = "Motif snapshot before tool execution"`) {
		t.Fatal("language not changed")
	}
	if err = integration.Uninstall(r, translated); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(m.Config.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(actual)) != strings.TrimSpace(userConfig) {
		t.Fatalf("user config changed: %s", actual)
	}
}
