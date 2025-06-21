package tui

import (
	"context"
	"encoding/json"
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
	cancel context.CancelFunc
	app    *tview.Application
	wait   chan struct{}
	notify <-chan *ctl.Message
}

var (
	instance  atomic.Pointer[ctlTuiClient]
	lastState atomic.Pointer[latency4go.State]
)

func msgLoop(instance *ctlTuiClient) {
	defer func() {
		instance.app.Stop()
		close(instance.wait)
	}()

	for msg := range instance.notify {
		var state *latency4go.State

		switch msg.GetType() {
		case ctl.MsgResult:
			result, err := msg.GetResult()

			if err != nil {
				slog.Error(
					"get result message failed",
					slog.Any("error", err),
				)
				continue
			}

			if result.Rtn != 0 {
				slog.Error(
					"command execution failed",
					slog.String("cmd", result.CmdName),
					slog.String("error_msg", result.Message),
				)
				continue
			}

			switch result.CmdName {
			case "state", "query":
				var rtn latency4go.State

				if err := json.Unmarshal(
					result.Values["state"].(json.RawMessage), &rtn,
				); err != nil {
					slog.Error(
						"unmarshal state failed",
						slog.Any("error", err),
					)
					continue
				}

				state = &rtn
			default:
				values := map[string]any{}

				for k, v := range result.Values {
					var value any

					if err := json.Unmarshal(
						v.(json.RawMessage), &value,
					); err != nil {
						slog.Error(
							"unmarshal result values failed",
							slog.Any("error", err),
							slog.String("key", k),
						)
					} else {
						values[k] = value
					}
				}

				slog.Info(
					"command result received",
					slog.String("cmd", result.CmdName),
					slog.Any("values", values),
				)
				continue
			}
		case ctl.MsgBroadCast:
			brd, err := msg.GetState()
			if err != nil {
				slog.Error(
					"get state message failed",
					slog.Any("error", err),
				)
				continue
			}
			state = brd
		default:
			slog.Warn(
				"unsupported return msg from ctl server",
				slog.Any("result", msg),
			)
			continue
		}

		if state != nil {
			slog.Info(
				"latency state notified",
				slog.Time("timestamp", state.Timestamp),
				slog.Any("config", state.Config),
			)

			lastState.Store(state)
			SetTopK()
			SetConfig()
		} else {
			slog.Error("state is empty")
		}
	}

	slog.Info("ctl client message loop exit")
}

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

	wait := make(chan struct{})
	tuiClient := &ctlTuiClient{
		client: client,
		flags:  flags,
		cancel: cancel,
		app:    app,
		notify: start(),
		wait:   wait,
	}

	instance.Store(tuiClient)

	go app.Run()

	go msgLoop(tuiClient)

	return wait, nil
}
