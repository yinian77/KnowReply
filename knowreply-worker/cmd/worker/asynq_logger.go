package main

import (
	"fmt"
	"log/slog"
	"os"
)

// asynqLogger 把 asynq 的日志并入结构化日志，避免和 slog 双份输出格式不一致。
type asynqLogger struct{ l *slog.Logger }

func newAsynqLogger(l *slog.Logger) *asynqLogger { return &asynqLogger{l: l} }

func (a *asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a *asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a *asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a *asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }

func (a *asynqLogger) Fatal(args ...any) {
	a.l.Error(fmt.Sprint(args...))
	os.Exit(1)
}
