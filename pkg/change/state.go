// Package change defines immutable file-state values and exact state transitions.
package change

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

type File struct {
	OutOfScope bool   `json:"out_of_scope,omitempty"`
	Mode       string `json:"mode"`
	Hash       string `json:"hash,omitempty"`
	OID        string `json:"oid,omitempty"`
}

// State keys are raw Git path bytes. Its JSON encoding preserves non-UTF-8 paths.
type State map[string]File

func (s State) MarshalJSON() ([]byte, error) {
	m := map[string]File{}
	for p, f := range s {
		m[base64.StdEncoding.EncodeToString([]byte(p))] = f
	}
	return json.Marshal(m)
}
func (s *State) UnmarshalJSON(b []byte) error {
	var m map[string]File
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*s = State{}
	for p, f := range m {
		raw, err := base64.StdEncoding.DecodeString(p)
		if err != nil {
			return err
		}
		(*s)[string(raw)] = f
	}
	return nil
}
func DisplayPath(p string) string {
	if utf8.ValidString(p) {
		return p
	}
	return fmt.Sprintf("%q", []byte(p))
}
func Equal(a, b State) bool {
	if len(a) != len(b) {
		return false
	}
	for p, f := range a {
		if g, ok := b[p]; !ok || g != f {
			return false
		}
	}
	return true
}
func Clone(s State) State {
	c := State{}
	for p, f := range s {
		c[p] = f
	}
	return c
}

// Apply uses exact file-state preconditions, rather than concatenating diff text.
func Apply(current, before, after State) (State, error) {
	out := Clone(current)
	paths := map[string]bool{}
	for p := range before {
		paths[p] = true
	}
	for p := range after {
		paths[p] = true
	}
	for p := range paths {
		b, bok := before[p]
		a, aok := after[p]
		if bok == aok && b == a {
			continue
		}
		c, cok := current[p]
		if cok != bok || c != b {
			return nil, fmt.Errorf("cannot replay path %s: recorded before state differs", DisplayPath(p))
		}
		if aok {
			out[p] = a
		} else {
			delete(out, p)
		}
	}
	return out, nil
}
