package content

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferencesResolveWithoutRegistration(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		raw, e := c.CombinedOutput()
		if e != nil {
			t.Fatal(string(raw), e)
		}
		return strings.TrimSpace(string(raw))
	}
	run("init", "-q")
	data := []byte("binary\x00data\n")
	file := filepath.Join(root, "blob")
	if e := os.WriteFile(file, data, 0600); e != nil {
		t.Fatal(e)
	}
	oid := run("hash-object", "-w", file)
	common := filepath.Join(root, ".git")
	objects := Objects{Dir: t.TempDir()}
	local, e := objects.Put(data)
	if e != nil {
		t.Fatal(e)
	}
	for _, ref := range []string{local, GitRef(common, oid)} {
		reopened := Objects{Dir: objects.Dir}
		got, e := reopened.Read(context.Background(), ref)
		if e != nil || !bytes.Equal(data, got) {
			t.Fatal("reference requires process registration", e)
		}
	}
	if GitIdentity(data, "sha1") != "sha1:"+oid {
		t.Fatal("local/Git byte identity differs")
	}
	p, _ := objects.Path(local)
	if e = os.WriteFile(p, []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = objects.Read(context.Background(), local); e == nil {
		t.Fatal("corrupt local content accepted")
	}
	if _, e = objects.Read(context.Background(), GitRef(common, strings.Repeat("0", 40))); e == nil {
		t.Fatal("missing Git object accepted")
	}
}
