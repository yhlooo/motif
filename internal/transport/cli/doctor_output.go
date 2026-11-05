package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/i18n"
)

func diagnosticStyle(color bool, code, text string) string {
	if !color || code == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func writeAgentDiagnostic(ctx context.Context, b *strings.Builder, agent string, d integration.Diagnostic, color bool) {
	t := func(message *goi18n.Message) string { return i18n.TContext(ctx, message) }
	paint := func(code, text string) string { return diagnosticStyle(color, code, text) }

	mark, code, summary := "!", "33", msgInstallUnknown
	var details []string
	switch d.Status {
	case "active", "configured":
		mark, code, summary = "✓", "32", msgInstallActive
		if agent == "claude" || agent == "codebuddy" {
			summary = msgInstallPending
		} else {
			details = append(details, t(msgInstallSessionOverrides))
		}
	case "inactive":
		summary = msgInstallInactive
	case "unavailable":
		summary = msgInstallUnavailable
	case "conflict", "missing", "unsupported":
		mark, code, summary = "✗", "31", msgInstallConflict
	}
	configurationOK := (d.Status == "active" || d.Status == "configured") && (agent == "claude" || agent == "codebuddy")
	if !configurationOK && len(details) == 0 && d.Detail != "" {
		details = append(details, d.Detail)
	}
	summaryCode := code
	if configurationOK {
		summaryCode = "2"
	}
	fmt.Fprintf(b, "    %s %-11s %s\n", paint(code, mark), installAgentName(agent), paint(summaryCode, t(summary)))
	for _, detail := range details {
		for _, line := range strings.Split(detail, "\n") {
			fmt.Fprintf(b, "      %s\n", line)
		}
	}
}

func writeDoctorResult(ctx context.Context, output io.Writer, root string, m integration.Manifest, agents []string, diagnostics map[string]integration.Diagnostic, color bool) error {
	var b strings.Builder
	t := func(message *goi18n.Message) string { return i18n.TContext(ctx, message) }
	fmt.Fprintf(&b, "\n  %s\n", diagnosticStyle(color, "1;36", t(msgDoctorTitle)))
	for _, agent := range agents {
		b.WriteByte('\n')
		d := diagnostics[agent]
		writeAgentDiagnostic(ctx, &b, agent, d, color)
		path := m.Config.Path
		if agent == "claude" && m.Claude != nil {
			path = m.Claude.Path
		}
		if agent == "codebuddy" && m.CodeBuddy != nil {
			path = m.CodeBuddy.Path
		}
		if path != "" {
			fmt.Fprintf(&b, "      %s\n", diagnosticStyle(color, "2", installDisplayPath(root, path)))
		}
		for _, hook := range d.Hooks {
			mark, code, enabled := "!", "33", msgDoctorDisabled
			if hook.Enabled {
				enabled = msgDoctorEnabled
				if hook.Trust == "trusted" || hook.Trust == "managed" {
					mark, code = "✓", "32"
				}
			}
			fmt.Fprintf(&b, "      %s %-16s %s  %s\n", diagnosticStyle(color, code, mark), hook.Event, t(enabled), hook.Trust)
		}
	}
	b.WriteByte('\n')
	_, err := io.WriteString(output, b.String())
	return err
}

var (
	msgDoctorTitle    = &goi18n.Message{ID: "commands.DoctorTitle", Other: "Hook diagnostics"}
	msgDoctorEnabled  = &goi18n.Message{ID: "commands.DoctorEnabled", Other: "enabled"}
	msgDoctorDisabled = &goi18n.Message{ID: "commands.DoctorDisabled", Other: "disabled"}
)
