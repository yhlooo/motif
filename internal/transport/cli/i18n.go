package commands

import "github.com/nicksnyder/go-i18n/v2/i18n"

var (
	MsgGlobalOptsVerbosityDesc = &i18n.Message{
		ID:    "commands.GlobalOptsVerbosityDesc",
		Other: "Number for the log level verbosity (0, 1, or 2)",
	}

	MsgRootDesc = &i18n.Message{
		ID:    "commands.CmdShortDesc",
		Other: "Intent-based code review for AI coding agents.",
	}
	MsgRootLongDesc = &i18n.Message{
		ID:    "commands.CmdLongDesc",
		Other: `Capture actual code changes, organize intent batches, and review them locally.`,
	}

	MsgVersionDesc = &i18n.Message{
		ID:    "commands.VersionDesc",
		Other: "Print the version information",
	}
	MsgVersionOptsOutputFormatDesc = &i18n.Message{
		ID:    "commands.VersionOptsOutputFormatDesc",
		Other: "Output format. One of (json, yaml)",
	}
)
