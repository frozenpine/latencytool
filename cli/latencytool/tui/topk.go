package tui

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/rivo/tview"
)

var (
	topKView = tview.NewFlex()
	topN     = tview.NewList()
	topTs    = tview.NewTextView()
	top      atomic.Uint32
	lastTop  atomic.Pointer[[]string]
)

func init() {
	top.Store(3)
	topKView.AddItem(
		topN.SetSelectedFocusOnly(true), 0, 5, false,
	).AddItem(
		topTs,
		0, 5, false,
	).SetTitle(
		fmt.Sprintf(" Top %d Fronts ", top.Load()),
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(
		true,
	).SetBorderPadding(
		1, 1, 1, 1,
	)
}

func change() {
	if client := instance.Load(); client != nil {
		client.app.Lock()
		defer client.app.Unlock()

		topN.Clear()

		for idx, v := range *lastTop.Load() {
			pri := idx + 1

			if pri > int(top.Load()) {
				break
			}

			priV := strconv.Itoa(pri)

			topN.AddItem(v, "", rune(priV[0]), nil)
		}

		topKView.SetTitle(fmt.Sprintf(" Top %d Fronts ", top.Load()))
		topTs.SetText(updateTs.Load().String())
	}
}

func SetTopK(values ...string) {
	lastTop.Store(&values)

	change()
}

func ChangeTopK(n int) {
	top.Store(uint32(n))

	change()
}
