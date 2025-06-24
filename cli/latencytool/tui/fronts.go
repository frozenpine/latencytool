package tui

import (
	"sync"
	"sync/atomic"

	"github.com/frozenpine/latency4go"
	"github.com/rivo/tview"
)

type hisStates struct {
	lock    sync.RWMutex
	history []*latency4go.State
}

func (h *hisStates) append(state *latency4go.State) {
	if state == nil {
		return
	}

	historicalTable.Box.GetRect()
	h.lock.Lock()
	h.history = append(h.history, state)

	h.lock.Unlock()
}

var (
	frontView       = tview.NewPages()
	historicalTable = tview.NewTable()

	history atomic.Pointer[hisStates]
)

func init() {
	history.Store(&hisStates{})
	frontView.AddPage(
		" Timed Quantile ", historicalTable, true, true,
	).SetTitle(
		" Front Historical ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(true)
}
