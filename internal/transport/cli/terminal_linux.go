package commands

import (
	"golang.org/x/sys/unix"
	"os"
)

func terminal() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return err == nil
}
