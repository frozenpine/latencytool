package tui

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/ctl"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/pflag"
)

type ctlTuiClient struct {
	client ctl.CtlClient
	flags  *pflag.FlagSet
	cancel func()
	app    *tview.Application
}

var (
	instance       atomic.Pointer[ctlTuiClient]
	lastState      atomic.Pointer[latency4go.State]
	stateCallbacks []func(*latency4go.State)
)

func setState(state *latency4go.State) *latency4go.State {
	if state == nil {
		return nil
	}

	old := lastState.Swap(state)
	for _, fn := range stateCallbacks {
		fn(state)
	}
	return old
}

func StartTui(
	ctx context.Context, client ctl.CtlClient,
	flags *pflag.FlagSet, cancel func(),
) error {
	if client == nil || flags == nil {
		return errors.New("invalid args")
	}

	app := tview.NewApplication().SetRoot(
		MainLayout, true,
	)
	exitFn := func() {
		app.Stop()
		cancel()
	}
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			commandView.SetText("")
			return nil
		case tcell.KeyTab:
			if commandView.HasFocus() {
				app.SetFocus(infoNodes)
			} else {
				app.SetFocus(commandView)
			}
		}
		return event
	})

	tuiClient := &ctlTuiClient{
		client: client,
		flags:  flags,
		cancel: exitFn,
		app:    app,
	}
	instance.Store(tuiClient)
	go app.Run()

	client.Init(ctx, "ctl client", client.Start)

	client.MessageLoop(
		"tui loop", nil,
		func(state *latency4go.State) error {
			if history.Load().append(setState(state)) {
				SetTopK()
				SetConfig()
			}

			slog.Info(
				"latency state notified",
				slog.Time("update_ts", state.Timestamp),
				slog.Any("priority", state.AddrList),
				slog.String("config", state.Config.String()),
			)
			return nil
		},
		func(r *ctl.Result) error {
			if r.CmdName == "state" {
				return nil
			}

			return ctl.LogResult(r)
		},
		func() error {
			app.Stop()
			return nil
		},
	)

	return nil
}
