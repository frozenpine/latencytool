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
	cancel context.CancelFunc
	app    *tview.Application
	wait   chan struct{}
}

var (
	instance  atomic.Pointer[ctlTuiClient]
	lastState atomic.Pointer[latency4go.State]
)

func StartTui(
	client ctl.CtlClient,
	flags *pflag.FlagSet,
	cancel context.CancelFunc,
) (<-chan struct{}, error) {
	if client == nil || flags == nil {
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

	wait := make(chan struct{})
	tuiClient := &ctlTuiClient{
		client: client,
		flags:  flags,
		cancel: cancel,
		app:    app,
	}
	client.MessageLoop(
		"tui loogp", nil,
		func(state *latency4go.State) error {
			lastState.Store(state)
			SetTopK()
			SetConfig()
			return nil
		},
		func() error {
			app.Stop()
			close(wait)
			return nil
		},
	)

	instance.Store(tuiClient)

	go app.Run()

	return wait, nil
}
