package gitrepo

import (
	"bytes"
	"context"
	"fmt"
)

// CommitMessages reads full messages for explicit commit OIDs without walking history.
func (r *Repository) CommitMessages(ctx context.Context, oids []string) (map[string]string, error) {
	messages := make(map[string]string, len(oids))
	unique := make([]string, 0, len(oids))
	for _, oid := range oids {
		if !ValidOID(oid) {
			return nil, fmt.Errorf("commit message requires full commit OID")
		}
		if _, exists := messages[oid]; !exists {
			messages[oid] = ""
			unique = append(unique, oid)
		}
	}
	// Bound command-line size while avoiding one Git process per commit.
	for start := 0; start < len(unique); start += 128 {
		end := min(start+128, len(unique))
		args := []string{"log", "--no-walk=unsorted", "--no-show-signature", "--no-notes", "--no-color", "--encoding=UTF-8", "--format=%H%x00%B", "-z"}
		args = append(args, unique[start:end]...)
		args = append(args, "--")
		raw, err := r.Git(ctx, args...)
		if err != nil {
			return nil, err
		}
		fields := bytes.Split(raw, []byte{0})
		if len(fields) != (end-start)*2+1 || len(fields[len(fields)-1]) != 0 {
			return nil, fmt.Errorf("invalid commit message records")
		}
		for i, oid := range unique[start:end] {
			if string(fields[i*2]) != oid {
				return nil, fmt.Errorf("unexpected commit message OID")
			}
			messages[oid] = string(fields[i*2+1])
		}
	}
	return messages, nil
}
