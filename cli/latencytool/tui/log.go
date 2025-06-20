package tui

import (
	"io"
	"os"

	"github.com/rivo/tview"
)

var logView = tview.NewTextView()

func init() {
	logView.Box.SetTitle(
		"Log",
	).SetTitleAlign(
		tview.AlignCenter,
	)
}

type tuiLogWr struct{}

func (wr *tuiLogWr) Write(data []byte) (int, error) {
	if ctlClient.Load() != nil {
		return logView.Write(data)
	}

	return os.Stderr.Write(data)
}

func LogWriter() io.Writer {
	return &tuiLogWr{}
}
