package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var errInstallCancelled = errors.New("installation cancelled")

type agentOption struct {
	name               string
	selected, detected bool
}

// terminalReader honors cancellation without leaving a goroutine reading stdin.
type terminalReader struct {
	ctx  context.Context
	file *os.File
}

type byteInput struct{ io.Reader }

func (r byteInput) ReadByte() (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r.Reader, b[:])
	return b[0], err
}

func (r terminalReader) Read(p []byte) (int, error) {
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		fds := []unix.PollFd{{Fd: int32(r.file.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 100)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n > 0 {
			return r.file.Read(p)
		}
	}
}

func installTerminal(cmd *cobra.Command) (*os.File, bool) {
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return nil, false
	}
	out, ok := cmd.OutOrStdout().(*os.File)
	return f, ok && term.IsTerminal(int(out.Fd())) && os.Getenv("TERM") != "dumb"
}

func chooseInstallAgents(ctx context.Context, input *os.File, output io.Writer) ([]string, error) {
	options := []agentOption{}
	for _, name := range []string{"codex", "claude", "codebuddy"} {
		_, err := exec.LookPath(name)
		options = append(options, agentOption{name: name, selected: err == nil, detected: err == nil})
	}
	state, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return nil, err
	}
	defer term.Restore(int(input.Fd()), state)
	// Keep redraws off the scrollback and always restore screen and cursor.
	if _, err = fmt.Fprint(output, "\x1b[?1049h\x1b[?25l"); err != nil {
		return nil, err
	}
	defer fmt.Fprint(output, "\x1b[?25h\x1b[?1049l")
	return selectInstallAgents(byteInput{terminalReader{ctx: ctx, file: input}}, output, options)
}

func selectInstallAgents(input io.ByteReader, output io.Writer, options []agentOption) ([]string, error) {
	cursor := 0
	for {
		var screen strings.Builder
		screen.WriteString("\x1b[H\x1b[2JChoose agents to install / repair\r\n\r\n")
		for i, option := range options {
			pointer, mark, hint := " ", " ", "CLI not found; can still configure"
			if i == cursor {
				pointer = ">"
			}
			if option.selected {
				mark = "x"
			}
			if option.detected {
				hint = "CLI detected"
			}
			fmt.Fprintf(&screen, "%s [%s] %s (%s)\r\n", pointer, mark, option.name, hint)
		}
		screen.WriteString("\r\nUp/Down: move  Space: toggle  Enter: confirm\r\nq / Ctrl+C: cancel\r\n")
		if _, err := io.WriteString(output, screen.String()); err != nil {
			return nil, err
		}
		key, err := input.ReadByte()
		if err != nil {
			return nil, err
		}
		switch key {
		case 3, 'q', 'Q':
			return nil, errInstallCancelled
		case ' ':
			options[cursor].selected = !options[cursor].selected
		case '\r', '\n':
			var selected []string
			for _, option := range options {
				if option.selected {
					selected = append(selected, option.name)
				}
			}
			return selected, nil
		case 27:
			prefix, err := input.ReadByte()
			if err != nil {
				return nil, err
			}
			if prefix != '[' && prefix != 'O' {
				continue
			}
			direction, err := input.ReadByte()
			if err != nil {
				return nil, err
			}
			if direction == 'A' {
				cursor = (cursor + len(options) - 1) % len(options)
			}
			if direction == 'B' {
				cursor = (cursor + 1) % len(options)
			}
		}
	}
}

func confirmInstall(input io.Reader, output io.Writer) error {
	if _, err := fmt.Fprint(output, "Apply these changes? [Y/n] "); err != nil {
		return err
	}
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "Y" {
		return errInstallCancelled
	}
	return nil
}
