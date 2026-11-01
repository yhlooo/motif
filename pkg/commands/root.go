package commands

import (
	"context"
	"fmt"

	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/yhlooo/motif/pkg/i18n"
	"github.com/yhlooo/motif/pkg/version"
)

// NewGlobalOptions 创建默认 GlobalOptions
func NewGlobalOptions() GlobalOptions {
	return GlobalOptions{
		Verbosity: 0,
	}
}

// GlobalOptions 全局选项
type GlobalOptions struct {
	// 日志数量级别（ 0 / 1 / 2 ）
	Verbosity uint32
}

// Validate 校验选项是否合法
func (o *GlobalOptions) Validate() error {
	if o.Verbosity > 2 {
		return fmt.Errorf("invalid log verbosity: %d (expected: 0, 1 or 2)", o.Verbosity)
	}
	return nil
}

// AddPFlags 将选项绑定到命令行参数
func (o *GlobalOptions) AddPFlags(fs *pflag.FlagSet) {
	fs.Uint32VarP(&o.Verbosity, "verbose", "v", o.Verbosity, i18n.T(MsgGlobalOptsVerbosityDesc))
}

type globalOptsContextKey struct{}

// ContextWithGlobalOptions 创建带全局选项的 context.Context
func ContextWithGlobalOptions(parent context.Context, opts GlobalOptions) context.Context {
	return context.WithValue(parent, globalOptsContextKey{}, opts)
}

// GlobalOptionsFromContext 从 ctx 获取全局选项
func GlobalOptionsFromContext(ctx context.Context) GlobalOptions {
	opts, _ := ctx.Value(globalOptsContextKey{}).(GlobalOptions)
	return opts
}

// NewOptions 创建默认 Options
func NewOptions() Options {
	return Options{}
}

// Options 运行选项
type Options struct {
}

// AddPFlags 将选项绑定到命令行参数
func (o *Options) AddPFlags(_ *pflag.FlagSet) {
}

// NewCommand 创建根命令
func NewCommand(name string) *cobra.Command {
	globalOpts := NewGlobalOptions()
	opts := NewOptions()

	cmd := &cobra.Command{
		Use:           name,
		Short:         i18n.T(MsgRootDesc),
		Long:          i18n.T(MsgRootLongDesc),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := globalOpts.Validate(); err != nil {
				return &UsageError{err}
			}
			ctx = ContextWithGlobalOptions(ctx, globalOpts)

			// 初始化 logger
			logrusLogger := logrus.New()
			switch globalOpts.Verbosity {
			case 0:
				logrusLogger.Level = logrus.InfoLevel
			case 1:
				logrusLogger.Level = logrus.DebugLevel
			default:
				logrusLogger.Level = logrus.TraceLevel
			}
			logger := logrusr.New(logrusLogger)
			ctx = logr.NewContext(ctx, logger)

			// 设置本地化器
			ctx = i18n.ContextWithLocalizer(ctx, i18n.NewLocalizer(i18n.GetEnvLanguage()))

			cmd.SetContext(ctx)

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},

		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	globalOpts.AddPFlags(cmd.PersistentFlags())
	opts.AddPFlags(cmd.Flags())

	cmd.AddCommand(
		newVersionCommand(),
		newHookCommand(), newPatchCommand(), newBatchCommand(),
		newInstallCommand(), newUninstallCommand(), newWebCommand(),
	)

	wrapUsage(cmd)
	return cmd
}
