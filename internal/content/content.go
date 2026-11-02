// Package content resolves durable content identities without callback registries.
package content

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRef preserves the repository location and object identity. It is independent
// of process caches and can be resolved after restarting the application.
func GitRef(common, oid string) string {
	return "git:" + base64.RawURLEncoding.EncodeToString([]byte(common)) + ":" + oid
}
func IsGit(ref string) bool   { return strings.HasPrefix(ref, "git:") }
func Hash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

type Objects struct{ Dir string }

func (o Objects) Path(ref string) (string, error) {
	if len(ref) != 64 {
		return "", fmt.Errorf("invalid local content reference")
	}
	if _, e := hex.DecodeString(ref); e != nil {
		return "", e
	}
	return filepath.Join(o.Dir, "blobs", "sha256", ref[:2], ref[2:]), nil
}
func (o Objects) Read(ctx context.Context, ref string) ([]byte, error) {
	release, e := o.Lease(false)
	if e != nil {
		return nil, e
	}
	defer release()
	if IsGit(ref) {
		parts := strings.Split(ref, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid Git content reference")
		}
		common, e := base64.RawURLEncoding.DecodeString(parts[1])
		if e != nil {
			return nil, e
		}
		if !filepath.IsAbs(string(common)) || (len(parts[2]) != 40 && len(parts[2]) != 64) {
			return nil, fmt.Errorf("invalid Git object reference")
		}
		if _, e = hex.DecodeString(parts[2]); e != nil {
			return nil, e
		}
		cmd := exec.CommandContext(ctx, "git", "--git-dir="+string(common), "cat-file", "blob", parts[2])
		data, e := cmd.Output()
		if e != nil {
			return nil, fmt.Errorf("Git object %s missing: %w", parts[2], e)
		}
		return data, nil
	}
	p, e := o.Path(ref)
	if e != nil {
		return nil, e
	}
	data, e := os.ReadFile(p)
	if e != nil {
		return nil, fmt.Errorf("content %s: %w", ref, e)
	}
	if Hash(data) != ref {
		return nil, fmt.Errorf("corrupt content %s", ref)
	}
	return data, nil
}
func (o Objects) Put(data []byte) (string, error) {
	release, e := o.Lease(false)
	if e != nil {
		return "", e
	}
	defer release()
	ref := Hash(data)
	p, _ := o.Path(ref)
	if old, e := os.ReadFile(p); e == nil {
		if !bytes.Equal(old, data) {
			return "", fmt.Errorf("corrupt content %s", ref)
		}
		return ref, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return "", e
	}
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return "", e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".object-")
	if e != nil {
		return "", e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return "", e
	}
	if e = os.Rename(f.Name(), p); e != nil {
		return "", e
	}
	dir, e := os.Open(filepath.Dir(p))
	if e != nil {
		return "", e
	}
	defer dir.Close()
	if e = dir.Sync(); e != nil {
		return "", e
	}
	return ref, nil
}

// GitIdentity is the byte identity used by the repository object format.
func GitIdentity(data []byte, format string) string {
	header := []byte(fmt.Sprintf("blob %d\x00", len(data)))
	if format == "sha256" {
		h := sha256.New()
		h.Write(header)
		h.Write(data)
		return "sha256:" + hex.EncodeToString(h.Sum(nil))
	}
	h := sha1.New()
	h.Write(header)
	h.Write(data)
	return "sha1:" + hex.EncodeToString(h.Sum(nil))
}
