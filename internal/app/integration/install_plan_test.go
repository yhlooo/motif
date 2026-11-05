package integration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/app/integration"
)

func TestInstallPreviewCancellationLeavesAllFilesUntouched(t *testing.T) {
	r, _ := installProject(t)
	path := filepath.Join(r.CommonDir, "hooks", "post-commit")
	original := "#!/bin/sh\necho original\n"
	if err := os.WriteFile(path, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	cancelled := errors.New("cancel")
	var preview []integration.FileChange
	_, changed, err := integration.InstallWithOptions(context.Background(), r, "/motif", integration.InstallOptions{
		Agents: []string{"codebuddy"}, BeforeApply: func(changes []integration.FileChange) error { preview = changes; return cancelled },
	})
	if !errors.Is(err, cancelled) || changed {
		t.Fatalf("%v %v", changed, err)
	}
	seen := map[string]bool{}
	backup := false
	for _, change := range preview {
		seen[change.Path] = true
		if strings.Contains(change.Path, "original-hooks") {
			backup = true
		}
		if change.Path != path {
			if _, err := os.Lstat(change.Path); !os.IsNotExist(err) {
				t.Fatalf("preview wrote %s: %v", change.Path, err)
			}
		}
	}
	if !seen[path] || !seen[filepath.Join(r.Root, ".codebuddy", "settings.local.json")] || !seen[filepath.Join(r.GitDir, "motif", "install.json")] || !backup {
		t.Fatal(preview)
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != original {
		t.Fatalf("%s %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, ".codebuddy")); !os.IsNotExist(err) {
		t.Fatal("cancelled preview created config directory", err)
	}
}

func TestInstallRejectsFilesEditedDuringConfirmation(t *testing.T) {
	r, _ := installProject(t)
	path := filepath.Join(r.Root, ".codebuddy", "settings.local.json")
	_, _, err := integration.InstallWithOptions(context.Background(), r, "/motif", integration.InstallOptions{
		Agents: []string{"codebuddy"}, BeforeApply: func(changes []integration.FileChange) error {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(`{"user":true}`), 0600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "changed after preview") {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != `{"user":true}` {
		t.Fatal(string(raw))
	}
	if _, err := integration.Load(r); !os.IsNotExist(err) {
		t.Fatal("installed despite stale preview", err)
	}
}

func TestInstallIdempotentPreviewIsEmpty(t *testing.T) {
	r, _ := installProject(t)
	options := integration.InstallOptions{Agents: []string{"claude", "codebuddy"}}
	if _, _, err := integration.InstallWithOptions(context.Background(), r, "/motif", options); err != nil {
		t.Fatal(err)
	}
	called := false
	options.BeforeApply = func(changes []integration.FileChange) error {
		called = true
		if len(changes) != 0 {
			t.Fatal(changes)
		}
		return nil
	}
	if _, changed, err := integration.InstallWithOptions(context.Background(), r, "/motif", options); err != nil || changed || !called {
		t.Fatalf("%v %v %v", changed, called, err)
	}
}
