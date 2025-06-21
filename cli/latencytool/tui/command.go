package tui

import (
	"log/slog"
	"strings"

	"github.com/frozenpine/latency4go/ctl"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/pflag"
)

var commandView = tview.NewInputField()

func init() {
	commandView.SetLabel(
		"Command > ",
	).SetDoneFunc(func(key tcell.Key) {
		client := ctlTuiClient.Load()

		if client == nil {
			return
		}

		inputCommand := commandView.GetText()
		if inputCommand == "" {
			return
		}

		commands := strings.Split(inputCommand, " ")
		kwargs := map[string]string{}

		cmdFlags := (*client.flags)

		if err := cmdFlags.ParseAll(
			commands[1:],
			func(flag *pflag.Flag, value string) error {
				if err := flag.Value.Set(value); err != nil {
					return err
				} else {
					kwargs[flag.Name] = value
					return nil
				}
			},
		); err != nil {
			slog.Error(
				"tui parse commands failed",
				slog.Any("error", err),
				slog.String("command", inputCommand),
			)
		}

		switch commands[0] {
		case "suspend":
		case "resume":
		case "interval":
		case "state":
		case "config":
		case "query":
		case "plugin":
		case "unplugin":
		default:
			slog.Error(
				"unsupported command",
				slog.String("cmd", commands[0]),
				slog.Any("args", commands[1:]),
			)
			return
		}

		if err := client.client.Command(&ctl.Command{
			Name:   commands[0],
			KwArgs: kwargs,
		}); err != nil {
			slog.Error(
				"send command failed",
				slog.Any("error", err),
				slog.String("cmd", commands[0]),
				slog.Any("args", commands[1:]),
			)
		}
	})
}
