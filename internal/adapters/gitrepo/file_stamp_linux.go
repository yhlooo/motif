package gitrepo

import (
	"os"
	"syscall"
)

func systemStamp(info os.FileInfo, st *fileStamp) {
	if raw, ok := info.Sys().(*syscall.Stat_t); ok {
		st.Ctime = raw.Ctim.Sec*1e9 + raw.Ctim.Nsec
		st.Device = uint64(raw.Dev)
		st.Inode = raw.Ino
	}
}
