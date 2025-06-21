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

var (
	MainLayout = tview.NewFlex()

	ctlTuiClient atomic.Pointer[struct {
		client ctl.CtlClient
		flags  *pflag.FlagSet
		cancel context.CancelFunc
		app    *tview.Application
	}]
)

func StartTui(
	client ctl.CtlClient,
	flags *pflag.FlagSet,
	cancel context.CancelFunc,
	notify <-chan *ctl.Message,
) (<-chan struct{}, error) {
	if client == nil || flags == nil || notify == nil {
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

	ctlTuiClient.Store(&struct {
		client ctl.CtlClient
		flags  *pflag.FlagSet
		cancel context.CancelFunc
		app    *tview.Application
	}{
		client: client,
		flags:  flags,
		cancel: cancel,
		app:    app,
	})

	go app.Run()

	wait := make(chan struct{})

	go func() {
		defer func() {
			app.Stop()
			close(wait)
		}()

		for msg := range notify {
			slog.Info(
				"message return from ctl server",
				slog.Any("msg", msg),
			)
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
	).SetTitle("LatencyTool").SetTitleAlign(tview.AlignCenter)
}
