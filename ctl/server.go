package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/libs"
	"github.com/frozenpine/msgqueue/channel"
)

var (
	ErrCtlServerArgs = errors.New("invalid ctl server args")
	ErrInitCtlServer = errors.New("init ctl server failed")
)

type CtlServer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	instance  *atomic.Pointer[latency4go.LatencyClient]
	initOnce  sync.Once
	startOnce sync.Once
	stopOnce  sync.Once
	handlers  []Handler
	broadcast channel.MemoChannel[*Message]
}

func NewCtlServer(
	ctx context.Context, cfg *CtlSvrHdlConfig,
) (svr *CtlServer, err error) {
	if cfg == nil {
		return nil, ErrCtlServerArgs
	}

	if ctx == nil {
		ctx = context.Background()
	}

	svr = &CtlServer{}

	svr.initOnce.Do(func() {
		svr.ctx, svr.cancel = context.WithCancel(ctx)

		svr.broadcast.Init(svr.ctx, "broadcast", nil)

		for _, hdl := range cfg.handlers {
			if hdl == nil {
				err = errors.New("nil ctl handler")
				return
			}

			hdl.Init(svr.ctx, hdl.Name(), hdl.Start)

			slog.Info(
				"connecting ctl handler broadcast",
				slog.String("hdl", hdl.Name()),
			)
			if err = svr.broadcast.PipelineDownStream(hdl); err != nil {
				slog.Error(
					"connect ctl handler broadcast failed",
					slog.Any("error", err),
					slog.String("hdl", hdl.Name()),
				)

				hdl.Release()
				return
			} else {
				slog.Info(
					"ctl handler broadcast connected",
					slog.String("hdl", hdl.Name()),
				)
			}

			svr.handlers = append(svr.handlers, hdl)

		}
	})

	return
}

func (svr *CtlServer) GetLatestState() *latency4go.State {
	return svr.instance.Load().GetLastState()
}

func (svr *CtlServer) Start(
	instance *atomic.Pointer[latency4go.LatencyClient],
) (err error) {
	if instance == nil {
		return fmt.Errorf(
			"%w: no latency client instance", ErrCtlServerArgs,
		)
	}

	svr.startOnce.Do(func() {
		svr.instance = instance

		if err = svr.instance.Load().AddReporter(
			"controller",
			func(state *latency4go.State) error {
				data, err := json.Marshal(state)

				if err != nil {
					slog.Error(
						"ctl reporter marshal state failed",
						slog.Any("error", err),
					)
				} else {
					if err = svr.broadcast.Publish(&Message{
						msgType: MsgBroadCast,
						data:    data,
					}, time.Second*5); err != nil {
						slog.Error("ctl reporter publish priorities timeout")
					} else {
						slog.Debug(
							"broadcasting state to ctl clients",
							slog.Any("state", state),
						)
					}
				}

				return nil
			},
		); err != nil {
			err = errors.Join(ErrInitCtlServer, err)

			return
		}

		go svr.runForever()
	})

	return
}

func (svr *CtlServer) Stop() {
	svr.stopOnce.Do(func() {
		svr.broadcast.Release()

		for _, hdl := range svr.handlers {
			hdl.Release()
		}

		svr.handlers = nil
	})
}

func (svr *CtlServer) Join() error {
	for _, hdl := range svr.handlers {
		hdl.Join()
	}

	return nil
}

func (svr *CtlServer) read() []reflect.SelectCase {
	cases := []reflect.SelectCase{
		{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(svr.ctx.Done()),
		},
	}

	return append(cases, latency4go.ConvertSlice(
		svr.handlers,
		func(hdl Handler) reflect.SelectCase {
			return reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(hdl.Commands()),
			}
		},
	)...)
}

func (svr *CtlServer) execute(msgID uint64, cmd *Command) (*Message, error) {
	slog.Info(
		"ctl server executing command",
		slog.Any("cmd", cmd),
	)
	switch cmd.Name {
	case "suspend":
		var msg string
		if svr.instance.Load().Suspend() {
			msg = "ok"
		} else {
			msg = "failed"
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    []byte(msg),
		}, nil
	case "resume":
		var msg string
		if svr.instance.Load().Resume() {
			msg = "ok"
		} else {
			msg = "failed"
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    []byte(msg),
		}, nil
	case "interval":
		intv, err := time.ParseDuration(cmd.KwArgs["interval"])
		if err != nil {
			return nil, err
		}

		rtn := svr.instance.Load().ChangeInterval(intv)
		if rtn <= 0 {
			return nil, fmt.Errorf("%w: invalid interval", ErrInvalidMsgData)
		}

		data, err := json.Marshal(map[string]any{
			"origin": rtn,
			"new":    intv,
		})
		if err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    data,
		}, nil
	case "state":
		state := svr.instance.Load().GetLastState()
		data, err := json.Marshal(state)

		if err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    data,
		}, nil
	case "config":
		if err := svr.instance.Load().SetConfig(cmd.KwArgs); err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		cfg := svr.instance.Load().GetConfig()
		data, err := json.Marshal(cfg)
		if err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    data,
		}, nil
	case "query":
		state, err := svr.instance.Load().QueryLatency(cmd.KwArgs)
		if err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		data, err := json.Marshal(state)
		if err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    data,
		}, nil
	case "plugin":
		name, exist := cmd.KwArgs["plugin"]
		if !exist {
			return nil, fmt.Errorf("%w: no plugin name", ErrInvalidMsgData)
		}
		config, exist := cmd.KwArgs["config"]
		if !exist {
			return nil, fmt.Errorf("%w: no plugin config", ErrInvalidMsgData)
		}
		libDir, exist := cmd.KwArgs["lib"]
		if !exist {
			return nil, fmt.Errorf("%w: no plugin base dir", ErrInvalidMsgData)
		}

		if container, err := libs.NewPlugin(libDir, name); err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		} else if err = container.Plugin().Init(svr.ctx, config); err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		} else if err = svr.instance.Load().AddReporter(
			name, func(s *latency4go.State) error {
				return container.Plugin().ReportFronts(s.AddrList...)
			},
		); err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		} else {
			return &Message{
				msgID:   msgID,
				msgType: MsgResult,
				data:    []byte(fmt.Sprintf("plugin[%s] added", name)),
			}, nil
		}
	case "unplugin":
		name, exist := cmd.KwArgs["plugin"]

		if !exist {
			return nil, fmt.Errorf("%w: no plugin name", ErrInvalidMsgData)
		}

		if err := svr.instance.Load().DelReporter(name); err != nil {
			return nil, fmt.Errorf(
				"%w: del reporter from client faield",
				ErrInvalidMsgData,
			)
		}

		container, err := libs.GetAndUnRegisterPlugin(name)
		if err != nil {
			if container == nil {
				return nil, errors.Join(ErrInvalidMsgData, err)
			} else {
				slog.Warn(
					"unregister plugin with error",
					slog.Any("error", err),
				)
			}
		}
		container.Plugin().Stop()
		if err = container.Plugin().Join(); err != nil {
			return nil, errors.Join(ErrInvalidMsgData, err)
		}

		return &Message{
			msgID:   msgID,
			msgType: MsgResult,
			data:    []byte(container.String()),
		}, nil
	default:
		return nil, errors.New("unsupported command")
	}
}

func (svr *CtlServer) write(idx int, msg *Message) error {
	if idx > len(svr.handlers) {
		return errors.New("handler not exists")
	}

	return svr.handlers[idx-1].Publish(msg, time.Second*3)
}

func (svr *CtlServer) runForever() {
	defer svr.Stop()

	for {
		select {
		case <-svr.ctx.Done():
			if err := svr.broadcast.Publish(&Message{
				msgType: MsgBroadCast,
				data:    []byte("server shutting down..."),
			}, time.Second*3); err != nil {
				slog.Error(
					"broadcasting exiting message failed",
					slog.Any("error", err),
				)
			}

			for _, hdl := range svr.handlers {
				hdl.Release()
			}
		default:
			idx, recv, ok := reflect.Select(svr.read())

			if !ok {
				slog.Info("message chan closed", slog.Int("idx", idx))

				// 0索引恒为ctx.Done()
				if idx == 0 {
					return
				} else {
					svr.handlers = slices.Delete(svr.handlers, idx, idx)
					continue
				}
			}

			slog.Debug(
				"message received from handler",
				slog.Int("idx", idx),
				slog.Any("value", recv),
			)

			if msg, ok := recv.Interface().(*Message); ok {
				cmd, err := msg.GetCommand()

				if err != nil {
					slog.Error(
						"receive a not commnd message",
						slog.Any("msg", recv.Interface()),
					)
				} else if rsp, err := svr.execute(msg.msgID, cmd); err != nil {
					slog.Error(
						"execute command failed",
						slog.Any("error", err),
						slog.Any("cmd", cmd),
					)
				} else if err := svr.write(idx, rsp); err != nil {
					slog.Error(
						"write message to handler failed",
						slog.Any("error", err),
					)
				}
			} else {
				slog.Error(
					"invalid message for handling",
					slog.Any("msg", recv.Interface()),
				)
			}
		}
	}
}
