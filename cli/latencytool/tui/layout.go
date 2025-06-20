package tui

import (
	"context"
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
		cancel context.CancelFunc
	}]
)

func StartTui(
	client ctl.CtlClient,
	flags *pflag.FlagSet,
	cancel context.CancelFunc,
) {
	ctlClient.Store(&struct {
		client ctl.CtlClient
		flags  *pflag.FlagSet
		cancel context.CancelFunc
	}{
		client: client,
		flags:  flags,
		cancel: cancel,
	})

	app := tview.NewApplication().SetRoot(MainLayout, true)

	go app.Run()
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
