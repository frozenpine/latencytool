package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	ctlSvrView = tview.NewFlex()
	summary    = tview.NewTextView()
	configNode = tview.NewTreeNode("Config")
	periodNode = tview.NewTreeNode("Period")
	pluginNode = tview.NewTreeNode("Plugins")
	infoNodes  = tview.NewTreeView()
)

func init() {
	ctlSvrView.SetDirection(
		tview.FlexRow,
	).AddItem(
		summary, 3, 0, false,
	).AddItem(
		infoNodes, 0, 1, false,
	).SetTitle(
		" Ctl Server Info ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(
		true,
	).SetBorderPadding(0, 0, 1, 1)

	summary.SetTitle(" Summary ").SetBorder(true)

	root := tview.NewTreeNode(
		"CtlServer",
	).Expand().AddChild(
		periodNode.SetColor(tcell.ColorDarkRed).Expand(),
	).AddChild(
		configNode.SetColor(tcell.ColorLightBlue).SetSelectable(true),
	).AddChild(
		pluginNode.SetColor(tcell.ColorDarkOrange).SetSelectable(true),
	)
	root.SetColor(tcell.ColorLightGreen).SetSelectable(true)

	infoNodes.SetRoot(root).SetTitle(
		" Info ",
	).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
		case tcell.KeyDown:
		case tcell.KeyLeft:
		case tcell.KeyRight:
		default:
			return event
		}

		return event
	})
}
