package ctl

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/libs"
)

type Command struct {
	Name   string
	KwArgs map[string]string
}

func (cmd *Command) Execute(svr *CtlServer) (result *Result, err error) {
	slog.Info(
		"executing command",
		slog.Any("cmd", cmd),
	)

	result = &Result{
		CmdName: cmd.Name,
		Values:  make(values),
	}

	client := svr.instance.Load()
	if client == nil && cmd.Name != "start" {
		result.Rtn = 1
		result.Message = "no latency client running"
		return
	}

	switch cmd.Name {
	case "suspend":
		if client.Suspend() {
			result.Message = "suspend success"
		} else {
			result.Rtn = 1
			result.Message = "suspend failed"
		}
	case "resume":
		if client.Resume() {
			result.Message = "resume success"
		} else {
			result.Rtn = 1
			result.Message = "resume failed"
		}
	case "stop":
		if err = svr.StopLatencyClient(); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"stop latency client failed: %+v", err,
			)
		} else {
			result.Message = "latency client stopped"
		}
	case "start":
		var newClient *latency4go.LatencyClient
		if newClient, err = svr.StartLatencyClient(cmd.KwArgs); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"start latency client failed: %+v", err,
			)
		} else {
			interval := newClient.GetInterval()

			plugins := []*libs.PluginContainer{}

			libs.RangePlugins(func(name string, container *libs.PluginContainer) error {
				plugins = append(plugins, container)
				return nil
			})

			result.Values[VKeyHandler] = latency4go.ConvertSlice(
				svr.handlers,
				func(h Handler) string {
					return h.ConnName()
				},
			)
			result.Values[VKeyInterval] = interval
			result.Values[VKeyPlugin] = plugins
			result.Message = "latency client started"
		}
	case "period":
		intv, err := time.ParseDuration(cmd.KwArgs["interval"])
		if err != nil {
			result.Rtn = 1
			result.Message = err.Error()
			break
		}

		rtn := client.ChangeInterval(intv)
		if rtn <= 0 {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: invalid interval", ErrInvalidMsgData,
			)
		}

		result.Values[VKeyIntervalOrigin] = rtn
		result.Values[VKeyInterval] = intv
		result.Message = "interval changed"
	case "state":
		if state := client.GetLastState(); state != nil {
			result.Values[VKeyState] = state
			result.Message = "get last state succeded"
		} else {
			result.Rtn = 1
			result.Message = "get last state failed"
		}
	case "config":
		if err = client.SetConfig(cmd.KwArgs); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: set config failed", err,
			)
			return
		}

		if cfg := client.GetConfig(); cfg != nil {
			result.Values[VKeyConfig] = cfg
			result.Message = "config set succeded"
		} else {
			result.Rtn = 1
			result.Message = "config setted, but no data return"
		}
	case "query":
		var state *latency4go.State
		state, err = client.QueryLatency(cmd.KwArgs)
		if err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: query latency failed", err,
			)

			return
		}

		if state != nil {
			result.Values[VKeyState] = state
			result.Message = "latency queried"
		} else {
			result.Rtn = 1
			result.Message = "latency query finished, but no state return"
		}
	case "plugin":
		name, exist := cmd.KwArgs["plugin"]
		if !exist {
			result.Rtn = 1
			result.Message = "no plugin name"
			err = fmt.Errorf("%w: no plugin name", ErrInvalidMsgData)
			return
		}

		config, exist := cmd.KwArgs["config"]
		if !exist {
			result.Rtn = 1
			result.Message = "no plugin config"
			err = fmt.Errorf("%w: no plugin config", ErrInvalidMsgData)
			return
		}
		libDir, exist := cmd.KwArgs["lib"]
		if !exist {
			result.Rtn = 1
			result.Message = "no plugin base dir"
			err = fmt.Errorf("%w: no plugin lib dir", ErrInvalidMsgData)
			return
		}

		var container *libs.PluginContainer
		if container, err = libs.NewPlugin(libDir, name); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"create plugin failed: %+v", err,
			)
			return
		}

		if err = container.Init(svr.ctx, config); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"init plugin failed: %+v", err,
			)
			return
		}

		if err = client.AddReporter(
			name, func(s *latency4go.State) error {
				return container.ReportFronts(s.AddrList...)
			},
		); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"add reporter failed: %+v", err,
			)
			return
		}

		result.Message = "new plugin added"
	case "unplugin":
		name, exist := cmd.KwArgs["plugin"]
		if !exist {
			result.Rtn = 1
			result.Message = "no plugin name"
			err = fmt.Errorf("%w: no plugin name", ErrInvalidMsgData)
			return
		}

		if err = client.DelReporter(name); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"del reporter from client faield: %+v", err,
			)
			return
		}

		var container *libs.PluginContainer
		container, err = libs.GetAndUnRegisterPlugin(name)
		if err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"get registered plugin failed: %+v", err,
			)
			return
		}

		container.Stop()
		if err = container.Join(); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: plugin stop failed", err,
			)
		} else {
			result.Message = "plugin unloaded"
		}
	case "info":
		if state := client.GetLastState(); state != nil {
			result.Values[VKeyState] = state
		}

		interval := client.GetInterval()

		plugins := []*libs.PluginContainer{}

		libs.RangePlugins(func(name string, container *libs.PluginContainer) error {
			plugins = append(plugins, container)
			return nil
		})

		result.Values[VKeyHandler] = latency4go.ConvertSlice(
			svr.handlers,
			func(h Handler) string {
				return h.ConnName()
			},
		)
		result.Values[VKeyInterval] = interval
		result.Values[VKeyPlugin] = plugins
		result.Message = "get info finished"
	default:
		result.Rtn = 1
		result.Message = "unsupported command"
	}

	return
}
