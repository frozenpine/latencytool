package tui

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/frozenpine/latency4go/ctl"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/pflag"
)

type ctlTuiClient struct {
	client ctl.CtlClient
	flags  *pflag.FlagSet
	cancel context.CancelFunc
	app    *tview.Application
}

var (
	MainLayout = tview.NewFlex()

	instance atomic.Pointer[ctlTuiClient]
)

func StartTui(
	client ctl.CtlClient,
	flags *pflag.FlagSet,
	cancel context.CancelFunc,
	start func() <-chan *ctl.Message,
) (<-chan struct{}, error) {
	if client == nil || flags == nil || start == nil {
		return nil, errors.New("invalid args")
	}

	app := tview.NewApplication().SetRoot(
		MainLayout, true,
	).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			cancel()
		}
		return event
	})

	instance.Store(&ctlTuiClient{
		client: client,
		flags:  flags,
		cancel: cancel,
		app:    app,
	})

	notify := start()

	go app.Run()

	wait := make(chan struct{})

	go func() {
		defer func() {
			app.Stop()
			close(wait)
		}()

		for msg := range notify {
			switch msg.GetType() {
			case ctl.MsgBroadCast:
				brd, err := msg.GetBroadCast()
				if err != nil {
					slog.Error(
						"get broadcast message failed",
						slog.Any("error", err),
					)
				} else {
					slog.Info(
						"latency result notified",
						slog.Any("timestamp", (*brd)["Timestamp"]),
						slog.Any("priority", (*brd)["AddrList"]),
						slog.Any("config", (*brd)["Config"]),
					)
				}
			case ctl.MsgResult:
				slog.Info(
					"message return from ctl server",
					slog.Any("result", msg),
				)
			default:
				slog.Warn(
					"unsupported return msg from ctl server",
					slog.Any("result", msg),
				)
			}
		}

		slog.Info("ctl client message loop exit")
	}()

	return wait, nil
}

func init() {
	MainLayout.SetFullScreen(true).SetDirection(
		tview.FlexRow,
	).AddItem(
		tview.NewFlex().AddItem(
			addrView, 0, 2, false,
		).AddItem(
			tview.NewFlex().SetDirection(
				tview.FlexRow,
			).AddItem(
				frontView, 0, 6, false,
			),
			0, 6, false,
		).AddItem(
			tview.NewFlex().AddItem(
				topKView, 0, 5, false,
			).AddItem(
				configView, 0, 5, false,
			),
			0, 2, false,
		), 0, 5, false,
	).AddItem(
		logView, 0, 5, false,
	).AddItem(
		commandView, 1, 0, true,
	).SetTitle(
		"LatencyTool",
	).SetTitleAlign(
		tview.AlignCenter,
	)
}
