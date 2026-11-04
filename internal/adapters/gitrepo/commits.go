package gitrepo

import (
	"bytes"
	"context"
	"fmt"
)

// CommitTitles reads subjects for explicit commit OIDs without walking history.
func (r *Repository) CommitTitles(ctx context.Context, oids []string) (map[string]string, error) {
	titles := make(map[string]string, len(oids))
	unique := make([]string, 0, len(oids))
	for _, oid := range oids {
		if !ValidOID(oid) {
			return nil, fmt.Errorf("commit title requires full commit OID")
		}
		if _, exists := titles[oid]; !exists {
			titles[oid] = ""
			unique = append(unique, oid)
		}
	}
	// Bound command-line size while avoiding one Git process per commit.
	for start := 0; start < len(unique); start += 128 {
		end := min(start+128, len(unique))
		args := []string{"log", "--no-walk=unsorted", "--no-show-signature", "--no-notes", "--no-color", "--encoding=UTF-8", "--format=%H%x00%s", "-z"}
		args = append(args, unique[start:end]...)
		args = append(args, "--")
		raw, err := r.Git(ctx, args...)
		if err != nil {
			return nil, err
		}
		fields := bytes.Split(raw, []byte{0})
		if len(fields) != (end-start)*2+1 || len(fields[len(fields)-1]) != 0 {
			return nil, fmt.Errorf("invalid commit title records")
		}
		for i, oid := range unique[start:end] {
			if string(fields[i*2]) != oid {
				return nil, fmt.Errorf("unexpected commit title OID")
			}
			titles[oid] = string(fields[i*2+1])
		}
	}
	return titles, nil
}
