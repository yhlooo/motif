package content

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yhlooo/motif/internal/domain/state"
)

// LoadBaseline resolves only immutable tree metadata. It never reads blob contents.
func LoadBaseline(ctx context.Context, ref state.BaselineRef) (state.State, error) {
	if !filepath.IsAbs(ref.CommonDir) || (ref.ObjectFormat != "sha1" && ref.ObjectFormat != "sha256") || (ref.Tree != "" && !validBaselineOID(ref.Tree)) {
		return state.State{}, fmt.Errorf("invalid Git baseline")
	}
	if ref.Tree == "" {
		return state.FromBaseline(ref, nil), nil
	}
	cmd := exec.CommandContext(ctx, "git", "--git-dir="+ref.CommonDir, "ls-tree", "-rz", "--full-tree", ref.Tree)
	raw, e := cmd.Output()
	if e != nil {
		return state.State{}, e
	}
	entries := map[string]state.File{}
	for _, record := range bytes.Split(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		header, path, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || !validBaselinePath(string(path)) {
			return state.State{}, fmt.Errorf("invalid Git baseline entry")
		}
		f := state.File{Mode: fields[0]}
		if f.Mode == "160000" {
			f.OID = fields[2]
		} else {
			f.ContentRef = GitRef(ref.CommonDir, fields[2])
			f.ContentID = ref.ObjectFormat + ":" + fields[2]
		}
		entries[string(path)] = f
	}
	return state.FromBaseline(ref, entries), nil
}

func validBaselineOID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	_, e := hex.DecodeString(id)
	return e == nil
}
func validBaselinePath(p string) bool {
	return p != "" && !filepath.IsAbs(p) && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator)) && p != ".git" && !strings.HasPrefix(p, ".git/")
}
