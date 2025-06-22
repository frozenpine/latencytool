package tui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/frozenpine/latency4go/ctl"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/pflag"
)

var (
	commandView = tview.NewInputField()

	commandHelp = `
Available commands:
--------------- Remote Commands -------------------
 suspend: suspend latency tool running
  resume: resume suspended latency tool
  period: change latency tool query period
   state: get latency tool last state
  config: change latency tool query config
   query: query latency result with onetime config
  plugin: add latency reporter plugin
unplugin: remove reporter plugin from latency tool
    show: show latency tool info
----------------- Local Commands ------------------
	help: print this help message
	 top: change TopK view
	exit: exit ctl client running

`

	suspendDetail = `
Commnad > suspend ↵

`
	resumeDetail = `
Commnad > resume ↵

`
	periodDetail = `
Commnad > period --interval {duration} ↵

`
	stateDetail = `
Commnad > state ↵

`
	configDetail = `
Commnad > config [--before {duration}] [--range {from=YYYY-mm-ddTHH:MM:SS[,to=YYYY-mm-ddTHH:MM:SS]}]
                 [--from {pico sec}] [--to {pico sec}] [--agg {result count}] [--least {agg least count}]
		         [--sort {parmas.(mid|avg|stdev|sample_stdev) +-*/ ...}]
		         [--user {client_id}]+ [--percents {quantile}]+ ↵

`
	queryDetail = `
Commnad > query [--before {duration}] [--range {from=YYYY-mm-ddTHH:MM:SS[,to=YYYY-mm-ddTHH:MM:SS]}]
                [--from {pico sec}] [--to {pico sec}] [--agg {result count}] [--least {agg least count}]
		        [--sort {parmas.(mid|avg|stdev|sample_stdev) +-*/ ...}]
		        [--user {client_id}]+ [--percents {quantile}]+ ↵

`
	pluginDetail = `
Commnad > plugin --name {plugin_name} --config {plugin_name}={config_file} ↵

`
	unpluginDetail = `
Commnad > unplugin --name {plugin_name}	↵

`
	showDetail = `
Commnad > show {something} ↵

`
	topDetail = `
Commnad > top {N} ↵

`
	exitDetail = `
Commnad > exit ↵

`

	commandDetails = map[string]string{
		"suspend":  suspendDetail,
		"resume":   resumeDetail,
		"period":   periodDetail,
		"state":    stateDetail,
		"config":   configDetail,
		"query":    queryDetail,
		"plugin":   pluginDetail,
		"unplugin": unpluginDetail,
		"show":     showDetail,
		"help":     commandHelp,
		"top":      topDetail,
		"exit":     exitDetail,
	}

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
			return
		}

		switch commands[0] {
		case "suspend":
		case "resume":
		case "period":
		case "state":
		case "config":
		case "query":
		case "plugin":
		case "unplugin":
		case "help":
			helpCmd := cmdFlags.Arg(0)
			if helpCmd == "" {
				logView.Write([]byte(commandHelp))
			} else {
				if cmdDetail, exists := commandDetails[helpCmd]; exists {
					logView.Write([]byte(cmdDetail))
				} else {
					slog.Error(
						"no command detail help found",
						slog.String("cmd", helpCmd),
					)
					logView.Write([]byte(commandHelp))
				}
			}
			goto END
		case "top":
			v := cmdFlags.Arg(0)
			n, err := strconv.Atoi(v)
			if err != nil {
				slog.Error(
					"invalid top number",
				)
			} else {
				ChangeTopK(n)
			}
			return
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
			if seq := client.client.GetCmdSeq(); seq > 1 {
				commandView.SetLabel(
					fmt.Sprintf("Command[%d] > ", seq),
				)
			} else {
				commandView.SetLabel("Command > ")
			}
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
