package ctl

import (
	"log/slog"
	"strings"
)

type CtlSvrHdlConfig struct {
	handlers []Handler
}

func (cfg *CtlSvrHdlConfig) Ipc(conn string) *CtlSvrHdlConfig {
	if cfg == nil {
		return nil
	}

	slog.Info("creating ipc ctl handler", slog.String("conn", conn))

	if ipc, err := NewIpcCtlHandler(
		strings.TrimPrefix(conn, "ipc://"),
	); err != nil {
		slog.Error(
			"create ipc handler failed",
			slog.Any("error", err),
		)

		return nil
	} else {
		cfg.handlers = append(cfg.handlers, ipc)
		return cfg
	}
}

func (cfg *CtlSvrHdlConfig) Tcp(conn string) *CtlSvrHdlConfig {
	if cfg == nil {
		return nil
	}

	slog.Info("creating tcp ctl handler", slog.String("conn", conn))

	if tcp, err := NewCtlTcpHandler(
		strings.TrimPrefix(conn, "tcp://"),
	); err != nil {
		slog.Error(
			"create tcp ctl handler failed",
			slog.Any("error", err),
		)

		return nil
	} else {
		cfg.handlers = append(cfg.handlers, tcp)

		return cfg
	}
}
