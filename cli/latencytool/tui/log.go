package tui

import (
	"io"
	"os"

	"github.com/rivo/tview"
)

var logView = tview.NewTextView()

func init() {
	logView.SetDynamicColors(
		true,
	).SetRegions(
		true,
	).SetWordWrap(
		true,
	).SetMaxLines(
		32,
	).SetChangedFunc(func() {
		if client := ctlTuiClient.Load(); client != nil {
			client.app.Draw()
		}
	}).SetBorder(
		true,
	).SetTitle(
		"Log",
	).SetTitleAlign(
		tview.AlignCenter,
	)
}

type tuiLogWr struct {
	buffer [][]byte
}

func (wr *tuiLogWr) Write(data []byte) (int, error) {
	if ctlTuiClient.Load() != nil {
		wr.buffer = append(wr.buffer, data)
		return logView.Write(data)
	}

	return os.Stderr.Write(data)
}

func LogWriter() io.Writer {
	return &tuiLogWr{}
}
