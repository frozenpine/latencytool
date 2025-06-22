package tui

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	ctlSvrView = tview.NewFlex()
	summary    = tview.NewTextView()
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
		periodNode.SetColor(
			tcell.ColorDarkRed,
		).Expand(),
	).AddChild(
		pluginNode.SetColor(
			tcell.ColorDarkOrange,
		).SetSelectable(true),
	)
	root.SetColor(tcell.ColorLightGreen).SetSelectable(true)

	infoNodes.SetRoot(
		root,
	).SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}

		switch node.GetText() {
		case "Period":
			interv, ok := ref.(time.Duration)
			if !ok {
				slog.Error(
					"invalid reference for Config node",
					slog.Any("ref", ref),
				)
				return
			}

			node.ClearChildren()
			node.AddChild(
				tview.NewTreeNode(
					fmt.Sprintf("Interval: %s", interv.String()),
				),
			)
		case "Plugins":
		}
	}).SetTitle(
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
