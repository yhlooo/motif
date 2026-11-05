package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// JSONHookConfig records only Motif's hook groups, never the user's whole settings.
type JSONHookConfig struct {
	File
	Created bool `json:"created,omitempty"`
}

func readHookSettings(path, agent string) (map[string]any, bool, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("refuse to replace non-regular %s config: %s", agent, path)
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !json.Valid(raw) {
		return nil, false, fmt.Errorf("invalid %s settings JSON", agent)
	}
	var settings map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&settings); err != nil {
		return nil, false, err
	}
	if settings == nil {
		return nil, false, fmt.Errorf("%s settings must be a JSON object", agent)
	}
	return settings, false, nil
}

// stripHookSettings removes exact owned groups. Changed/duplicated managed entries
// are conflicts; unrelated settings and hooks remain owned by the user.
func stripHookSettings(settings map[string]any, old *JSONHookConfig, allowMissing bool, agent string) error {
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
			return fmt.Errorf("%s hooks must be an object", agent)
		}
	}
	counts := map[string]int{}
	for event, value := range hooks {
		groups, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s hook groups must be arrays: %s", agent, event)
		}
		kept := []any{}
		for _, group := range groups {
			encoded, _ := json.Marshal(group)
			owned := strings.Contains(string(encoded), "motif:"+agent) || strings.Contains(string(encoded), " hook "+agent)
			if owned {
				if !reflect.DeepEqual(group, expected[event]) {
					return fmt.Errorf("modified or unregistered Motif %s hook: %s", agent, event)
				}
				counts[event]++
				if counts[event] > 1 {
					return fmt.Errorf("duplicate Motif %s hook: %s", agent, event)
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
				return fmt.Errorf("Motif %s hook is missing: %s", agent, event)
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

func prepareJSONHooks(path, agent string, groups map[string]any, old *JSONHookConfig) (*JSONHookConfig, []byte, error) {
	settings, created, err := readHookSettings(path, agent)
	if err != nil {
		return nil, nil, err
	}
	if err = stripHookSettings(settings, old, true, agent); err != nil {
		return nil, nil, err
	}
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
	return &JSONHookConfig{File: File{Path: path, Content: string(owned)}, Created: created}, append(next, '\n'), err
}

func removeJSONHooks(config *JSONHookConfig, allowMissing bool, agent string) ([]byte, error) {
	settings, missing, err := readHookSettings(config.Path, agent)
	if err != nil {
		return nil, err
	}
	if missing && !allowMissing {
		return nil, os.ErrNotExist
	}
	if err = stripHookSettings(settings, config, allowMissing, agent); err != nil {
		return nil, err
	}
	if config.Created && len(settings) == 0 {
		return nil, nil
	}
	next, err := json.MarshalIndent(settings, "", "  ")
	return append(next, '\n'), err
}
