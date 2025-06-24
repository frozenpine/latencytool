package tui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/rivo/tview"
)

var configView = tview.NewList()

func init() {
	configView.SetSelectedFocusOnly(
		true,
	).SetBorder(
		true,
	).SetTitle(
		" Latency Config ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorderPadding(
		0, 0, 1, 1,
	)
}

func SetConfig() {
	if client := instance.Load(); client != nil {
		state := lastState.Load()

		if state == nil {
			slog.Error("no state found when set Config")
			return
		}

		client.app.Lock()

		configView.Clear()

		configView.AddItem(
			"TimeRange", state.Config.TimeRange.String(), '*', nil,
		).AddItem(
			"Tick2Order", state.Config.Tick2Order.String(), '*', nil,
		).AddItem(
			"Agg", fmt.Sprintf(
				"size: %d, least: %d",
				state.Config.AggSize, state.Config.AggCount,
			), '*', nil,
		).AddItem(
			"Quantile", state.Config.Quantile.String(), '*', nil,
		).AddItem(
			"Users", state.Config.Users.String(), '*', nil,
		).AddItem(
			"SortBy",
			strings.ReplaceAll(state.Config.SortBy, "params.", ""),
			'*', nil,
		)

		client.app.Unlock()

		client.app.Draw()
	}
}
