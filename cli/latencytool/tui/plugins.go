package tui

import "github.com/rivo/tview"

var pluginsView = tview.NewTreeView()

func init() {
	pluginsView.SetTitle(
		" Plugins ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(true)
}
