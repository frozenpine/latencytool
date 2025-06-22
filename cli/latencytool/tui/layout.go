package tui

import (
	"github.com/rivo/tview"
)

var MainLayout = tview.NewFlex()

func init() {
	MainLayout.SetFullScreen(true).SetDirection(
		tview.FlexRow,
	).AddItem(
		tview.NewFlex().AddItem(
			pluginsView, 0, 2, false,
		).AddItem(
			frontView, 0, 6, false,
		).AddItem(
			tview.NewFlex().SetDirection(
				tview.FlexRow,
			).AddItem(
				topKView, 0, 6, false,
			).AddItem(
				configView, 0, 4, false,
			),
			0, 2, false,
		),
		0, 5, false,
	).AddItem(
		logView, 0, 5, false,
	).AddItem(
		commandView, 1, 0, true,
	).SetTitle(
		"LatencyTool",
	).SetTitleAlign(
		tview.AlignCenter,
	)
}
