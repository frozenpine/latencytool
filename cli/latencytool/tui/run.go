package tui

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

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

func handleState(state *latency4go.State) error {
	if state == nil {
		return nil
	}

	history.Load().append(lastState.Swap(state))
	SetTopK()
	SetConfig()

	slog.Info(
		"latency state notified",
		slog.Time("update_ts", state.Timestamp),
		slog.Any("priority", state.AddrList),
		slog.String("config", state.Config.String()),
	)
	return nil
}

func handleResultInfo(r *ctl.Result) error {
	stateV, ok := r.Values["State"].(json.RawMessage)
	if !ok {
		slog.Error("no state in info result")
	} else {
		var state latency4go.State
		if err := json.Unmarshal(stateV, &state); err != nil {
			return err
		} else if err := handleState(&state); err != nil {
			return err
		}
	}

	interV, ok := r.Values["Interval"].(json.RawMessage)
	if !ok {
		slog.Error("no interval in info result")
	} else {
		var interval time.Duration
		if err := json.Unmarshal(interV, &interval); err != nil {
			return err
		}

		SetInterval(interval)
	}

	hdlV, ok := r.Values["Handlers"].(json.RawMessage)
	if !ok {
		slog.Error("no handlers in info result")
	} else {
		var handlers = []string{}
		if err := json.Unmarshal(hdlV, &handlers); err != nil {
			return err
		}

		SetSummary(handlers)
	}

	return nil
}

func handleResultPeriod(r *ctl.Result) error {
	ctl.LogResult(r)

	newV, ok := r.Values["New"].(json.RawMessage)
	if !ok {
		slog.Error("no new interval in period result")
	} else {
		var interv time.Duration
		if err := json.Unmarshal(newV, &interv); err != nil {
			return err
		}

		SetInterval(interv)
	}

	return nil
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
			// case tcell.KeyTab:
			// 	if commandView.HasFocus() {
			// 		app.SetFocus(infoNodes)
			// 	} else {
			// 		app.SetFocus(commandView)
			// 	}
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
		handleState,
		func(r *ctl.Result) error {
			switch r.CmdName {
			case "info":
				return handleResultInfo(r)
			case "period":
				return handleResultPeriod(r)
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
