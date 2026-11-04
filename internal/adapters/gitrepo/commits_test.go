package gitrepo_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/adapters/gitrepo"
)

func TestCommitTitlesExplicitOIDs(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		raw, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, raw, err)
		}
		return strings.TrimSpace(string(raw))
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	git("commit", "--allow-empty", "-m", "第一项：<script> & 标题", "-m", "body must not become the title")
	first := git("rev-parse", "HEAD")
	git("commit", "--allow-empty", "-m", "second")
	second := git("rev-parse", "HEAD")
	git("commit", "--allow-empty", "--allow-empty-message", "-m", "")
	empty := git("rev-parse", "HEAD")
	r := &gitrepo.Repository{Root: root}
	titles, err := r.CommitTitles(context.Background(), []string{second, empty, first, first})
	if err != nil || len(titles) != 3 || titles[first] != "第一项：<script> & 标题" || titles[second] != "second" || titles[empty] != "" {
		t.Fatalf("unexpected titles: %v, %v", titles, err)
	}
	if r.ReadStats().GitCalls != 1 {
		t.Fatal("metadata was not read in a batch")
	}
	if _, err := r.CommitTitles(context.Background(), []string{"HEAD"}); err == nil {
		t.Fatal("mutable revision accepted")
	}
	before := r.ReadStats().GitCalls
	if titles, err := r.CommitTitles(context.Background(), nil); err != nil || len(titles) != 0 || r.ReadStats().GitCalls != before {
		t.Fatal("empty request must not read HEAD", titles, err)
	}
}
