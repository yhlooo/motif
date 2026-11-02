package cli

import (
	"golang.org/x/sys/unix"
	"os"
)

func terminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
	return err == nil
}
