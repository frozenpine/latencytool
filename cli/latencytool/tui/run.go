package tui

import (
	"context"
	"errors"
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
	instance  atomic.Pointer[ctlTuiClient]
	lastState atomic.Pointer[latency4go.State]
)

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
			exitFn()
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
			lastState.Store(state)
			SetTopK()
			SetConfig()
			return nil
		},
		ctl.LogResult,
		func() error {
			app.Stop()
			return nil
		},
	)

	return nil
}
