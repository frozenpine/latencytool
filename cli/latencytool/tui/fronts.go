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

	h.lock.Lock()
	defer h.lock.Unlock()

	h.history = append(h.history, state)
}

var (
	frontView = tview.NewTable()

	history atomic.Pointer[hisStates]
)

func init() {
	history.Store(&hisStates{})
	frontView.SetTitle(
		" Front Historical ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(true)
}
