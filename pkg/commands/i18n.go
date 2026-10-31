package commands

import "github.com/nicksnyder/go-i18n/v2/i18n"

var (
	MsgGlobalOptsVerbosityDesc = &i18n.Message{
		ID:    "commands.GlobalOptsVerbosityDesc",
		Other: "Number for the log level verbosity (0, 1, or 2)",
	}

	MsgRootDesc = &i18n.Message{
		ID:    "commands.CmdShortDesc",
		Other: "Example Go application.",
	}
	MsgRootLongDesc = &i18n.Message{
		ID:    "commands.CmdLongDesc",
		Other: `This is an example go application.`,
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
