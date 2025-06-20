package tui

import (
	"sync/atomic"

	"github.com/rivo/tview"
)

var (
	MainLayout = tview.NewFlex()

	running atomic.Bool
)

func init() {
	MainLayout.SetFullScreen(true).SetDirection(
		tview.FlexRow,
	).AddItem(
		tview.NewFlex().AddItem(
			addrView, 0, 2, false,
		).AddItem(
			tview.NewFlex().SetDirection(
				tview.FlexRow,
			).AddItem(
				frontView, 0, 6, false,
			).AddItem(logView, 0, 4, false),
			0, 6, false,
		).AddItem(
			tview.NewFlex().AddItem(
				topKView, 0, 5, false,
			).AddItem(
				configView, 0, 5, false,
			),
			0, 2, false,
		), 0, 8, false,
	).AddItem(
		commandView, 0, 2, true,
	).SetTitle("LatencyTool").SetTitleAlign(tview.AlignCenter)
}
