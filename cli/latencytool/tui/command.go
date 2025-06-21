package tui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/frozenpine/latency4go/ctl"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/pflag"
)

var (
	commandView = tview.NewInputField()

	commandHelp = `Available commands:
 suspend: suspend latency client running
  resume: resume suspended latency client
interval: change latency tool query interval
   state: query latency tool last state
  config: change latency tool query config
   query: query latency result with onetime config
  plugin: add latency reporter plugin
unplugin: remove reporter plugin from latency tool
	help: print this help message
	exit: exit ctl client running
`
	commandHistory = []string{}
	commandHisIdx  = 0
)

func init() {
	commandView.SetLabel(
		"Command > ",
	).SetFieldStyle(
		tcell.StyleDefault.Foreground(
			tcell.ColorWhite,
		).Background(
			tcell.ColorBlack,
		),
	).SetDoneFunc(func(key tcell.Key) {
		client := instance.Load()

		if client == nil {
			return
		}

		inputCommand := strings.TrimSpace(commandView.GetText())
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
		case "help":
			logView.Write([]byte(commandHelp))
			goto END
		case "exit":
			client.cancel()
			return
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

	END:
		commandHistory = append(commandHistory, inputCommand)
		commandHisIdx = len(commandHistory)
	}).SetFinishedFunc(func(key tcell.Key) {
		commandView.SetText("")

		if client := instance.Load(); client != nil {
			commandView.SetLabel(
				fmt.Sprintf("Command[%d] > ", client.client.GetCmdSeq()),
			)
		}
	})

	commandView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			commandHisIdx--
			if commandHisIdx < 0 {
				commandHisIdx = -1
				commandView.SetText("")
				return nil
			}
		case tcell.KeyDown:
			commandHisIdx++
			if commandHisIdx >= len(commandHistory) {
				commandHisIdx = len(commandHistory)
				commandView.SetText("")
				return nil
			}
		default:
			return event
		}

		if commandHisIdx >= 0 && commandHisIdx < len(commandHistory) {
			commandView.SetText(commandHistory[commandHisIdx])
		} else {
			commandView.SetText("")
		}

		return nil
	})
}
