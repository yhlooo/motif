//go:build !linux && !darwin

package gitrepo

import "os"

// Unknown timestamp layouts conservatively disable cached content reuse.
func systemStamp(_ os.FileInfo, _ *fileStamp) {}
