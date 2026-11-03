package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/yhlooo/motif/internal/app/workspace"
	"github.com/yhlooo/motif/internal/i18n"
)

// ClaudeConfig records only Motif's hook groups, never the user's whole settings.
type ClaudeConfig struct {
	File
	Created bool `json:"created,omitempty"`
}

func claudeGroups(ctx context.Context, binary string) map[string]any {
	groups := map[string]any{}
	for _, hook := range codexHooks {
		if hook.event == "Interrupt" {
			continue
		} // Claude has no Interrupt hook.
		groups[hook.event] = map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": quote(binary) + " hook claude || true # motif:claude",
			"timeout": 30, "async": false, "statusMessage": i18n.TContext(ctx, hook.statusMessage),
		}}}
	}
	// Failed commands can still have changed files; close the matching call too.
	groups["PostToolUseFailure"] = groups["PostToolUse"]
	groups["StopFailure"] = groups["SessionEnd"]
	return groups
}

func readClaude(path string) (map[string]any, bool, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("refuse to replace non-regular Claude config: %s", path)
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !json.Valid(raw) {
		return nil, false, fmt.Errorf("invalid Claude settings JSON")
	}
	var settings map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&settings); err != nil {
		return nil, false, err
	}
	if settings == nil {
		return nil, false, fmt.Errorf("Claude settings must be a JSON object")
	}
	return settings, false, nil
}

// stripClaude removes exact owned groups. Changed/duplicated managed entries
// are conflicts; unrelated settings and hooks remain owned by the user.
func stripClaude(settings map[string]any, old *ClaudeConfig, allowMissing bool) error {
	expected := map[string]any{}
	if old != nil {
		decoder := json.NewDecoder(strings.NewReader(old.Content))
		decoder.UseNumber()
		if err := decoder.Decode(&expected); err != nil {
			return err
		}
	}
	hooks := map[string]any{}
	if value, ok := settings["hooks"]; ok {
		var valid bool
		hooks, valid = value.(map[string]any)
		if !valid {
			return fmt.Errorf("Claude hooks must be an object")
		}
	}
	counts := map[string]int{}
	for event, value := range hooks {
		groups, ok := value.([]any)
		if !ok {
			return fmt.Errorf("Claude hook groups must be arrays: %s", event)
		}
		kept := []any{}
		for _, group := range groups {
			encoded, _ := json.Marshal(group)
			owned := strings.Contains(string(encoded), "motif:claude") || strings.Contains(string(encoded), " hook claude")
			if owned {
				if !reflect.DeepEqual(group, expected[event]) {
					return fmt.Errorf("modified or unregistered Motif Claude hook: %s", event)
				}
				counts[event]++
				if counts[event] > 1 {
					return fmt.Errorf("duplicate Motif Claude hook: %s", event)
				}
			} else {
				kept = append(kept, group)
			}
		}
		if len(kept) == 0 && counts[event] > 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if !allowMissing {
		for event := range expected {
			if counts[event] != 1 {
				return fmt.Errorf("Motif Claude hook is missing: %s", event)
			}
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
	return nil
}

func prepareClaude(ctx context.Context, r *workspace.Project, binary string, old *ClaudeConfig) (*ClaudeConfig, []byte, error) {
	path := filepath.Join(r.Root, ".claude", "settings.local.json")
	settings, created, err := readClaude(path)
	if err != nil {
		return nil, nil, err
	}
	if err = stripClaude(settings, old, true); err != nil {
		return nil, nil, err
	}
	groups := claudeGroups(ctx, binary)
	// Serialize only the groups owned by Motif.
	owned, err := json.Marshal(groups)
	if err != nil {
		return nil, nil, err
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for event, group := range groups {
		list, _ := hooks[event].([]any)
		hooks[event] = append(list, group)
	}
	settings["hooks"] = hooks
	next, err := json.MarshalIndent(settings, "", "  ")
	if old != nil {
		created = old.Created
	}
	return &ClaudeConfig{File: File{Path: path, Content: string(owned)}, Created: created}, append(next, '\n'), err
}

func removeClaude(config *ClaudeConfig, allowMissing bool) ([]byte, error) {
	settings, missing, err := readClaude(config.Path)
	if err != nil {
		return nil, err
	}
	if missing && !allowMissing {
		return nil, os.ErrNotExist
	}
	if err = stripClaude(settings, config, allowMissing); err != nil {
		return nil, err
	}
	if config.Created && len(settings) == 0 {
		return nil, nil
	}
	next, err := json.MarshalIndent(settings, "", "  ")
	return append(next, '\n'), err
}
