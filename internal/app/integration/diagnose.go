package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yhlooo/motif/internal/app/workspace"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type HookState struct {
	Event   string `json:"event"`
	Enabled bool   `json:"enabled"`
	Trust   string `json:"trust"`
}
type Diagnostic struct {
	Status string      `json:"status"`
	Detail string      `json:"detail"`
	Hooks  []HookState `json:"hooks"`
}

// Diagnose reads the installed Codex CLI's effective hooks. It does not run a
// model, execute a hook, change project trust or bypass the CLI's trust policy.
func Diagnose(ctx context.Context, r *workspace.Project, m Manifest) Diagnostic {
	binary, err := exec.LookPath("codex")
	if err != nil {
		return Diagnostic{Status: "unavailable", Detail: "Codex CLI 未安装；配置已生成，尚未检查是否生效。", Hooks: []HookState{}}
	}
	if err = Verify(RemovalPlan(m, []string{"codex"})); err != nil {
		return Diagnostic{Status: "conflict", Detail: err.Error(), Hooks: []HookState{}}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--no-daemon", "app-server", "--listen", "stdio://")
	cmd.Dir = r.Root
	cmd.Stderr = io.Discard
	input, err := cmd.StdinPipe()
	if err != nil {
		return unavailable(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return unavailable(err)
	}
	if err = cmd.Start(); err != nil {
		return unavailable(err)
	}
	defer func() { input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	encoder, decoder := json.NewEncoder(input), json.NewDecoder(output)
	send := func(id int, method string, params any) error {
		return encoder.Encode(map[string]any{"id": id, "method": method, "params": params})
	}
	receive := func(id int, target any) error {
		for {
			var envelope struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := decoder.Decode(&envelope); err != nil {
				return err
			}
			if envelope.ID != id {
				continue
			}
			if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
				return fmt.Errorf("Codex CLI 不支持或拒绝 hooks/list 配置查询")
			}
			if target != nil {
				return json.Unmarshal(envelope.Result, target)
			}
			return nil
		}
	}
	if err = send(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "motif", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}}); err != nil {
		return unavailable(err)
	}
	if err = receive(1, nil); err != nil {
		return unavailable(err)
	}
	if err = encoder.Encode(map[string]any{"method": "initialized"}); err != nil {
		return unavailable(err)
	}
	if err = send(2, "hooks/list", map[string]any{"cwds": []string{r.Root}}); err != nil {
		return unavailable(err)
	}
	var result struct {
		Data []struct {
			CWD      string            `json:"cwd"`
			Errors   []json.RawMessage `json:"errors"`
			Warnings []string          `json:"warnings"`
			Hooks    []struct {
				Command    string `json:"command"`
				Event      string `json:"eventName"`
				SourcePath string `json:"sourcePath"`
				Enabled    bool   `json:"enabled"`
				Trust      string `json:"trustStatus"`
				Async      bool   `json:"async"`
			} `json:"hooks"`
		} `json:"data"`
	}
	if err = receive(2, &result); err != nil {
		return unavailable(err)
	}
	var config struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `toml:"command"`
			} `toml:"hooks"`
		} `toml:"hooks"`
	}
	if err = toml.Unmarshal([]byte(m.Config.Content), &config); err != nil {
		return unavailable(err)
	}
	expected := map[string]string{}
	for event, groups := range config.Hooks {
		if len(groups) > 0 && len(groups[0].Hooks) > 0 {
			expected[strings.ToLower(event[:1])+event[1:]] = groups[0].Hooks[0].Command
		}
	}
	diagnostic := Diagnostic{Status: "active", Detail: "Codex 已加载 Motif 的同步 hooks，且信任状态允许执行。当前会话的额外覆盖仍以该会话为准。", Hooks: []HookState{}}
	for event, command := range expected {
		state := HookState{Event: event, Trust: "missing"}
		for _, entry := range result.Data {
			if filepath.Clean(entry.CWD) != filepath.Clean(r.Root) {
				continue
			}
			for _, hook := range entry.Hooks {
				if filepath.Clean(hook.SourcePath) == filepath.Clean(m.Config.Path) && hook.Command == command && hook.Event == event {
					state.Enabled = hook.Enabled && !hook.Async
					state.Trust = hook.Trust
				}
			}
		}
		diagnostic.Hooks = append(diagnostic.Hooks, state)
		if !state.Enabled || state.Trust != "trusted" && state.Trust != "managed" {
			diagnostic.Status = "inactive"
			diagnostic.Detail = "配置已写入，但 Codex 未加载、未启用或尚未信任部分 Motif hooks；不能视为采集已生效。请在 Codex 中检查项目配置和 hook 信任状态。"
		}
	}
	return diagnostic
}
func unavailable(err error) Diagnostic {
	return Diagnostic{Status: "unavailable", Detail: "无法通过当前 Codex CLI 查询生效配置；请检查 CLI 版本与配置。尚未确认采集生效。", Hooks: []HookState{}}
}

// DiagnoseAgent checks each integration according to its supported health check.
// Valid managed hook configuration is sufficient for Claude Code and CodeBuddy.
func DiagnoseAgent(ctx context.Context, r *workspace.Project, m Manifest, agent string) Diagnostic {
	if agent == "codex" {
		return Diagnose(ctx, r, m)
	}
	if agent == "codebuddy" {
		if m.CodeBuddy == nil {
			return Diagnostic{Status: "missing", Detail: "CodeBuddy integration is not installed.", Hooks: []HookState{}}
		}
		if _, err := removeJSONHooks(m.CodeBuddy, false, "codebuddy"); err != nil {
			return Diagnostic{Status: "conflict", Detail: err.Error(), Hooks: []HookState{}}
		}
		return Diagnostic{Status: "active", Detail: "CodeBuddy project hook configuration is valid. Restart the client to apply changes.", Hooks: []HookState{}}
	}
	if agent != "claude" {
		return Diagnostic{Status: "unsupported", Detail: "Unsupported agent: " + agent, Hooks: []HookState{}}
	}
	if m.Claude == nil {
		return Diagnostic{Status: "missing", Detail: "Claude Code integration is not installed.", Hooks: []HookState{}}
	}
	if _, err := removeClaude(m.Claude, false); err != nil {
		return Diagnostic{Status: "conflict", Detail: err.Error(), Hooks: []HookState{}}
	}
	return Diagnostic{Status: "active", Detail: "Claude Code project hook configuration is valid. Restart the client to apply changes.", Hooks: []HookState{}}
}
