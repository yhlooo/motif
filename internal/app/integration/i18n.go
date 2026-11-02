package integration

import "github.com/nicksnyder/go-i18n/v2/i18n"

var codexHooks = []struct {
	event         string
	statusMessage *i18n.Message
}{
	{"PreToolUse", &i18n.Message{ID: "integration.CodexPreToolUseStatus", Other: "Motif snapshot before tool execution"}},
	{"PostToolUse", &i18n.Message{ID: "integration.CodexPostToolUseStatus", Other: "Motif snapshot after tool execution"}},
	{"Stop", &i18n.Message{ID: "integration.CodexStopStatus", Other: "Motif snapshot at turn end"}},
	{"Interrupt", &i18n.Message{ID: "integration.CodexInterruptStatus", Other: "Motif snapshot on interruption"}},
	{"SessionEnd", &i18n.Message{ID: "integration.CodexSessionEndStatus", Other: "Motif snapshot at session end"}},
}
