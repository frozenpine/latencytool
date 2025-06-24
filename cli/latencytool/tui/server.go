package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	ctlSvrView = tview.NewFlex()
	summary    = tview.NewTextView()
	period     = tview.NewTextView()
	pluginNode = tview.NewTreeNode("Plugins")
	infoNodes  = tview.NewTreeView()
)

func SetInterval(dur time.Duration) {
	if client := instance.Load(); client != nil {
		client.app.Lock()
		period.SetText(fmt.Sprintf(`["1"][orange]%s[""][white]`, dur.String()))
		period.Highlight("1")
		client.app.Unlock()

		client.app.Draw()
	}
}

func SetSummary(values []string) {
	if client := instance.Load(); client != nil {
		client.app.Lock()
		summary.Clear()
		for idx, v := range values {
			if idx > 0 {
				summary.Write([]byte("\n"))
			}
			summary.Write([]byte(v))
		}
		client.app.Unlock()

		client.app.Draw()
	}
}

func init() {
	ctlSvrView.SetDirection(
		tview.FlexRow,
	).AddItem(
		summary, 0, 3, false,
	).AddItem(
		period, 3, 0, false,
	).AddItem(
		infoNodes, 0, 7, false,
	).SetTitle(
		" Ctl Server Info ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(
		true,
	).SetBorderPadding(0, 0, 1, 1)

	summary.SetTitle(
		" Summary ",
	).SetBorder(
		true,
	).SetBorderPadding(
		0, 0, 1, 1,
	)
	period.SetDynamicColors(
		true,
	).SetRegions(
		true,
	).SetTitle(
		" Query Interval ",
	).SetTitleAlign(
		tview.AlignCenter,
	).SetBorder(
		true,
	).SetBorderPadding(
		0, 0, 1, 1,
	)

	root := tview.NewTreeNode(
		"CtlServer",
	).Expand().AddChild(
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
		case "Plugins":
			// TODO expand child
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
