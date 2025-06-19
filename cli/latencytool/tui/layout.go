package tui

import "github.com/rivo/tview"

var (
	MainLayout *tview.Flex
)

func init() {
	MainLayout = tview.NewFlex()

	MainLayout.SetFullScreen(true).
		SetDirection(tview.FlexRow).
		SetBorder(true).
		SetTitle("LatencyTool").
		SetTitleAlign(tview.AlignCenter)
}
