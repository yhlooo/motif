package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yhlooo/motif/internal/transport/cli"
)

func main() {
	ctx, cancel := NotifySignals(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cmd := cli.NewCommand("motif")
	if err := cmd.ExecuteContext(ctx); err != nil {
		log.Print(err)
		os.Exit(cli.ExitCode(err))
	}
}

// NotifySignals 将信号绑定到上下文
func NotifySignals(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	// 绑定信号通知
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	if ctx.Err() == nil {
		// 监听信号
		go func() {
			// 第一次收到信号取消上下文
			select {
			case <-ctx.Done():
				return
			case <-ch:
				cancel()
			}
			// 第二次直接退出
			select {
			case s, ok := <-ch:
				if !ok || s == nil {
					os.Exit(1)
				}
				if syscallSignal, isSyscallSignal := s.(syscall.Signal); isSyscallSignal {
					os.Exit(128 + int(syscallSignal)) // 128+n 被信号终止的退出码
				}
				os.Exit(1)
			}
		}()
	}

	return ctx, cancel
}
