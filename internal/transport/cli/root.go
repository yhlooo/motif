package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/yhlooo/motif/internal/i18n"
	"github.com/yhlooo/motif/internal/version"
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
	Verbosity uint32 `json:"verbosity"`
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

		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	globalOpts.AddPFlags(cmd.PersistentFlags())
	opts.AddPFlags(cmd.Flags())

	cmd.AddCommand(
		newVersionCommand(),
		newHookCommand(), newPatchCommand(), newBatchCommand(),
		newInstallCommand(), newUninstallCommand(), newWebCommand(), newDoctorCommand(),
	)

	wrapRuntime(cmd, &globalOpts)
	wrapUsage(cmd)
	return cmd
}

func wrapRuntime(cmd *cobra.Command, flags *GlobalOptions) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			dir, err := dataDir()
			if err != nil {
				return err
			}
			opts := NewGlobalOptions()
			if cmd.Flags().Changed("verbose") {
				opts.Verbosity = flags.Verbosity
			} else {
				path := filepath.Join(dir, "config.json")
				raw, err := os.ReadFile(path)
				if err == nil {
					if err := json.Unmarshal(raw, &opts); err != nil {
						return fmt.Errorf("read %s: %w", path, err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			if err := opts.Validate(); err != nil {
				return &UsageError{err}
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				return err
			}
			file, err := os.OpenFile(filepath.Join(dir, "motif.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer file.Close()

			logger := logrus.New()
			logger.SetOutput(file)
			logger.SetLevel([]logrus.Level{logrus.InfoLevel, logrus.DebugLevel, logrus.TraceLevel}[opts.Verbosity])

			ctx := ContextWithGlobalOptions(cmd.Context(), opts)
			ctx = logr.NewContext(ctx, logrusr.New(logger))
			ctx = i18n.ContextWithLocalizer(ctx, i18n.NewLocalizer(i18n.GetEnvLanguage()))
			cmd.SetContext(ctx)
			return run(cmd, args)
		}
	}
	for _, child := range cmd.Commands() {
		wrapRuntime(child, flags)
	}
}
