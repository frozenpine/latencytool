package tui

import (
	"sync/atomic"

	"github.com/frozenpine/latency4go/ctl"
	"github.com/rivo/tview"
	"github.com/spf13/pflag"
)

var (
	MainLayout = tview.NewFlex()

	ctlClient atomic.Pointer[struct {
		client ctl.CtlClient
		flags  *pflag.FlagSet
	}]
)

func StartTui(client ctl.CtlClient, flags *pflag.FlagSet) error {
	ctlClient.Store(&struct {
		client ctl.CtlClient
		flags  *pflag.FlagSet
	}{
		client: client,
		flags:  flags,
	})

	return nil
}

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
