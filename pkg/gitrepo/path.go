package gitrepo

import "encoding/base64"

func encodePath(p string) string { return base64.StdEncoding.EncodeToString([]byte(p)) }
