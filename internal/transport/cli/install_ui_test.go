package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestInstallMultiSelect(t *testing.T) {
	for _, tc := range []struct {
		name, keys string
		want       []string
		err        error
	}{
		{"navigate-toggle", " \x1b[B \x1b[B \x1b[A \r", []string{"codex", "codebuddy"}, nil},
		{"wrap-up", "\x1b[A \n", []string{"codebuddy"}, nil},
		{"application-arrows", "\x1bOB \r", []string{"claude"}, nil},
		{"toggle-off", "  \r", nil, nil},
		{"cancel", "q", nil, errInstallCancelled},
		{"interrupt", "\x03", nil, errInstallCancelled},
		{"eof", "", nil, io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := selectInstallAgents(bufio.NewReader(strings.NewReader(tc.keys)), &output, []agentOption{{name: "codex"}, {name: "claude"}, {name: "codebuddy"}})
			if !errors.Is(err, tc.err) || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%v %v; want %v %v", got, err, tc.want, tc.err)
			}
			if !strings.Contains(output.String(), "Space: toggle") {
				t.Fatal(output.String())
			}
		})
	}
}

func TestInstallSelectionDoesNotConsumeConfirmation(t *testing.T) {
	input := strings.NewReader(" \rY\n")
	_, err := selectInstallAgents(byteInput{input}, io.Discard, []agentOption{{name: "codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = confirmInstall(input, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFinalConfirmation(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "n\n", "N\n", "\n", " \n", "yes\n", ""} {
		var output bytes.Buffer
		err := confirmInstall(strings.NewReader(input), &output)
		if !strings.Contains(output.String(), "[Y/n]") {
			t.Fatal(output.String())
		}
		want := strings.TrimSpace(input) == "Y"
		if (err == nil) != want {
			t.Fatalf("%q: %v", input, err)
		}
	}
}
