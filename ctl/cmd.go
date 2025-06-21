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

func (cmd *Command) Execute(svr *CtlServer) (*Result, error) {
	slog.Info(
		"executing command",
		slog.Any("cmd", cmd),
	)

	result := Result{
		CmdName: cmd.Name,
	}

	switch cmd.Name {
	case "suspend":
		if svr.instance.Load().Suspend() {
			result.Message = "suspend success"
		} else {
			result.Rtn = 1
			result.Message = "suspend failed"
		}
	case "resume":
		if svr.instance.Load().Resume() {
			result.Message = "resume success"
		} else {
			result.Rtn = 1
			result.Message = "resume failed"
		}
	case "period":
		intv, err := time.ParseDuration(cmd.KwArgs["interval"])
		if err != nil {
			result.Rtn = 1
			result.Message = err.Error()
			break
		}

		rtn := svr.instance.Load().ChangeInterval(intv)
		if rtn <= 0 {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: invalid interval", ErrInvalidMsgData,
			)
		}

		result.Values = map[string]any{
			"origin": rtn.String(),
			"new":    intv.String(),
		}
	case "state":
		if state := svr.instance.Load().GetLastState(); state != nil {
			result.Values = map[string]any{
				"state": state,
			}
		} else {
			result.Rtn = 1
			result.Message = "get last state failed"
		}
	case "config":
		if err := svr.instance.Load().SetConfig(cmd.KwArgs); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: set config failed", err,
			)
			break
		}

		cfg := svr.instance.Load().GetConfig()
		result.Values = map[string]any{
			"Config": cfg,
		}
	case "query":
		state, err := svr.instance.Load().QueryLatency(cmd.KwArgs)
		if err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: query latency failed", err,
			)

			break
		}

		result.Values = map[string]any{
			"state": state,
		}
	case "plugin":
		name, exist := cmd.KwArgs["plugin"]
		if !exist {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: no plugin name", ErrInvalidMsgData,
			)
			break
		}

		config, exist := cmd.KwArgs["config"]
		if !exist {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: no plugin config", ErrInvalidMsgData,
			)
			break
		}
		libDir, exist := cmd.KwArgs["lib"]
		if !exist {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: no plugin base dir", ErrInvalidMsgData,
			)
		}

		if container, err := libs.NewPlugin(libDir, name); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: create plugin failed", err,
			)
		} else if err = container.Plugin().Init(svr.ctx, config); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: init plugin failed", err,
			)
		} else if err = svr.instance.Load().AddReporter(
			name, func(s *latency4go.State) error {
				return container.Plugin().ReportFronts(s.AddrList...)
			},
		); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: add reporter failed", err,
			)
		}
	case "unplugin":
		name, exist := cmd.KwArgs["plugin"]

		if !exist {
			result.Rtn = 1
			result.Message = "no plugin name"
			break
		}

		if err := svr.instance.Load().DelReporter(name); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: del reporter from client faield", err,
			)
			break
		}

		container, err := libs.GetAndUnRegisterPlugin(name)
		if err != nil {
			if container == nil {
				result.Rtn = 1
				result.Message = fmt.Sprintf(
					"%+v: get registered plugin failed", err,
				)
				break
			} else {
				slog.Warn(
					"unregister plugin with error",
					slog.Any("error", err),
				)
			}
		}

		container.Plugin().Stop()
		if err = container.Plugin().Join(); err != nil {
			result.Rtn = 1
			result.Message = fmt.Sprintf(
				"%+v: plugin stop failed", err,
			)
		}
	default:
		result.Rtn = 1
		result.Message = "unsupported command"
	}

	return &result, nil
}
