package tui

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"

	"github.com/rivo/tview"
)

var (
	topKView = tview.NewFlex()
	topN     = tview.NewList()
	topTs    = tview.NewTextView()
	top      atomic.Uint32
)

func init() {
	top.Store(6)

	topKView.SetDirection(
		tview.FlexRow,
	).AddItem(
		topN.SetSelectedFocusOnly(true), 0, 1, false,
	).AddItem(
		topTs, 1, 0, false,
	).SetTitle(
		fmt.Sprintf(" Top %d Fronts ", top.Load()),
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(
		true,
	).SetBorderPadding(
		0, 0, 1, 1,
	)

	topTs.SetRegions(
		true,
	).SetDynamicColors(
		true,
	).SetToggleHighlights(
		true,
	)
}

func SetTopK() {
	if client := instance.Load(); client != nil {
		state := lastState.Load()
		if state == nil {
			slog.Error("no state found when set TopK")
			return
		}

		client.app.Lock()
		defer client.app.Unlock()

		topN.Clear()

		for idx, v := range state.AddrList {
			pri := idx + 1

			if pri > int(top.Load()) {
				break
			}

			priV := strconv.Itoa(pri)

			topN.AddItem(
				v,
				"priority: "+strconv.FormatFloat(
					state.LatencyList[idx].Priority,
					'f', -1, 64,
				),
				rune(priV[0]),
				nil)
		}

		topKView.SetTitle(fmt.Sprintf(" Top %d Fronts ", top.Load()))
		topTs.SetText(fmt.Sprintf(
			"[\"1\"]%s[\"\"]",
			state.Timestamp.Local().Format("2006-01-02 15:04:05"),
		))
		topTs.Highlight("1")
	}
}

func ChangeTopK(n int) {
	top.Store(uint32(n))

	SetTopK()
}
