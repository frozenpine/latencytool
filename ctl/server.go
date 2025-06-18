package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/frozenpine/latency4go"
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

type CtlSvrHdlConfig struct {
	handlers []Handler
}

func (cfg *CtlSvrHdlConfig) Ipc(conn string) *CtlSvrHdlConfig {
	if cfg == nil {
		return nil
	}

	slog.Info("creating ipc ctl handler", slog.String("conn", conn))

	if ipc, err := NewIpcCtlHandler(
		strings.TrimPrefix(conn, "ipc://"),
	); err != nil {
		slog.Error(
			"create ipc handler failed",
			slog.Any("error", err),
		)

		return nil
	} else {
		cfg.handlers = append(cfg.handlers, ipc)
		return cfg
	}
}

func (cfg *CtlSvrHdlConfig) Http(conn string) *CtlSvrHdlConfig {
	// TODO conn check
	// cfg.httpConn.conn = conn
	slog.Error("http ctl handler not implemented")
	return nil
}

func (cfg *CtlSvrHdlConfig) Tcp(conn string) *CtlSvrHdlConfig {
	// TODO conn check
	slog.Error("tcp ctl handler not implemented")
	return nil
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

			hdl.Init(svr.ctx, hdl.Name())
			hdl.Start()

			slog.Info(
				"connecting ctl handler broadcast",
				slog.String("hdl", hdl.Name()),
			)
			if err = svr.broadcast.PipelineDownStream(hdl.Results()); err != nil {
				slog.Error(
					"connect ctl handler broadcast failed",
					slog.Any("error", err),
					slog.String("hdl", hdl.Name()),
				)

				hdl.Stop()
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
			func(addrList ...string) error {
				data, err := json.Marshal(map[string][]string{
					"priorities": addrList,
				})

				if err != nil {
					slog.Error(
						"ctl reporter marshal addr list failed",
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
							"broadcasting priority list to ctl clients",
							slog.Any("addresses", addrList),
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
			hdl.Stop()
		}

		svr.handlers = nil
	})
}

func (svr *CtlServer) Join() error {
	errList := []error{}
	for _, hdl := range svr.handlers {
		if err := hdl.Join(); err != nil {
			errList = append(errList, err)
		}
	}

	return errors.Join(errList...)
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

func (svr *CtlServer) execute(cmd *Command) (*Message, error) {
	switch cmd.Name {
	// TODO cmd execution
	default:
		return nil, errors.New("unsupported command")
	}
}

func (svr *CtlServer) write(idx int, msg *Message) error {
	if idx >= len(svr.handlers) {
		return errors.New("handler not exists")
	}

	return svr.handlers[idx].Publish(msg, time.Second*3)
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
				hdl.Stop()
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

			if msg, ok := recv.Interface().(*Message); ok {
				cmd, err := msg.GetCommand()

				if err != nil {
					slog.Error(
						"receive a not commnd message",
						slog.Any("msg", recv.Interface()),
					)
				} else if rsp, err := svr.execute(cmd); err != nil {
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
