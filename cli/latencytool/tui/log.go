package tui

import (
	"bytes"
	"io"
	"os"

	"github.com/rivo/tview"
	"github.com/valyala/bytebufferpool"
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
		50,
	).SetChangedFunc(func() {
		if client := instance.Load(); client != nil {
			logView.Lock()
			logView.ScrollToEnd()
			logView.Highlight("0", "1")
			logView.Unlock()

			client.app.Draw()
		}
	}).SetBorder(
		true,
	).SetTitle(
		" Log ",
	).SetTitleAlign(
		tview.AlignCenter,
	)
}

type tuiLogWr struct{}

func (wr *tuiLogWr) Write(data []byte) (int, error) {
	if instance.Load() != nil {
		logBuffer := bytebufferpool.Get()
		defer bytebufferpool.Put(logBuffer)

		for word := range bytes.FieldsSeq(data) {
			switch {
			case bytes.HasPrefix(word, []byte("time=")):
				logBuffer.WriteString(`time=["0"][gray]`)
				logBuffer.Write(word[5:])
				logBuffer.WriteString(`[""][white]`)
			case bytes.HasPrefix(word, []byte("level=")):
				logBuffer.WriteString(`level=["1"]`)
				var color string
				switch {
				case bytes.HasSuffix(word, []byte("INFO")):
					color = `[green]`
				case bytes.HasSuffix(word, []byte("WARN")):
					color = `[orange]`
				case bytes.HasSuffix(word, []byte("ERROR")):
					color = `[red]`
				default:
					color = `[gray]`
				}
				logBuffer.WriteString(color)
				logBuffer.Write(word[6:])
				logBuffer.WriteString(`[""][white]`)
			default:
				logBuffer.Write(word)
			}

			logBuffer.WriteByte(' ')
		}
		logBuffer.WriteByte('\n')

		return logView.Write(logBuffer.Bytes())
	}

	return os.Stderr.Write(data)
}

func LogWriter() io.Writer {
	return &tuiLogWr{}
}
