package commands

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/yhlooo/motif/pkg/store"
)

type UsageError struct{ error }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage *UsageError
	if errors.As(err, &usage) || errors.Is(err, store.ErrInvalidArgument) {
		return 2
	}
	return 1
}
func wrapUsage(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &UsageError{err} })
	if check := cmd.Args; check != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := check(c, args); err != nil {
				return &UsageError{err}
			}
			return nil
		}
	}
	for _, child := range cmd.Commands() {
		wrapUsage(child)
	}
}
