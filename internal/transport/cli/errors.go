package cli

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/yhlooo/motif/internal/adapters/sqlite"
)

type UsageError struct{ error }

// HookFeedback is intentional Stop feedback, delivered on stderr with exit 2.
type HookFeedback struct{ Reason string }

func (e *HookFeedback) Error() string { return e.Reason }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var feedback *HookFeedback
	if errors.As(err, &feedback) {
		return 2
	}
	var usage *UsageError
	if errors.As(err, &usage) || errors.Is(err, sqlite.ErrInvalidArgument) {
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
