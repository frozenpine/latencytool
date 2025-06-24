package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"slices"
	"strconv"
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

	queryCfg      *latency4go.QueryConfig
	queryInterval time.Duration
	queryAddr     string
	querySink     string
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

func (svr *CtlServer) connectReporter() error {
	return svr.instance.Load().AddReporter(
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
	)
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

		if err = svr.connectReporter(); err != nil {
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

			msg, ok := recv.Interface().(*Message)
			if !ok {
				slog.Error(
					"invalid message for handling",
					slog.Any("msg", recv.Interface()),
				)
				continue
			}

			cmd, err := msg.GetCommand()

			if err != nil {
				slog.Error(
					"receive a not commnd message",
					slog.Any("msg", recv.Interface()),
				)
				continue
			}

			result, err := cmd.Execute(svr)
			if err != nil {
				slog.Error(
					"execute command failed",
					slog.Any("error", err),
					slog.Any("msg", msg),
				)
			}
			if result == nil {
				continue
			}

			data, err := json.Marshal(result)
			if err != nil {
				slog.Error(
					"marshal result failed",
					slog.Any("error", err),
					slog.Any("msg", msg),
					slog.Any("result", result),
				)
			}

			rsp := Message{
				msgID:   msg.msgID,
				msgType: MsgResult,
				data:    data,
			}

			if err := svr.write(idx, &rsp); err != nil {
				slog.Error(
					"write message to handler failed",
					slog.Any("error", err),
				)
			}

		}
	}
}

func (svr *CtlServer) StopLatencyClient() error {
	client := svr.instance.Load()
	if client == nil {
		return errors.New("no latency client")
	}

	defer svr.instance.Store(nil)

	// store running config for next start
	svr.queryCfg = client.GetConfig()
	svr.queryAddr = client.GetAddr()
	svr.querySink = client.GetSinkPath()
	svr.queryInterval = client.GetInterval()
	slog.Info("latency client last running config stored")

	client.Stop()

	return client.Join()
}

func (svr *CtlServer) StartLatencyClient(kwargs map[string]string) (*latency4go.LatencyClient, error) {
	client := svr.instance.Load()
	if client != nil {
		return nil, errors.New("latency client already started")
	}

	scheme, ok := kwargs["schema"]
	if ok {
		delete(kwargs, "schema")
	}
	host, ok := kwargs["host"]
	if ok {
		delete(kwargs, "host")
	}
	port, ok := kwargs["port"]
	if ok {
		delete(kwargs, "port")
	}

	var portN int
	if scheme == "" || host == "" || port == "" {
		if addr, err := url.Parse(svr.queryAddr); err != nil {
			return nil, err
		} else if portN, err = strconv.Atoi(addr.Port()); err != nil {
			return nil, err
		} else {
			scheme = addr.Scheme
			host = addr.Hostname()
		}
	}

	sink, ok := kwargs["sink"]
	if ok {
		delete(kwargs, "sink")
	} else {
		sink = svr.querySink
	}

	var inter time.Duration
	interV, ok := kwargs["interval"]
	if ok {
		delete(kwargs, "interval")

		var err error
		if inter, err = time.ParseDuration(interV); err != nil {
			return nil, err
		}
	} else {
		inter = svr.queryInterval
	}

	cfg := svr.queryCfg
	for k, v := range kwargs {
		if err := cfg.SetConfig(k, v); err != nil {
			return nil, err
		}
	}

	slog.Info(
		"initiating latency client with config",
		slog.String("schema", scheme),
		slog.String("host", host),
		slog.Int("port", portN),
		slog.String("sink", sink),
		slog.Any("query_cfg", cfg),
	)

	client = &latency4go.LatencyClient{}
	if err := client.Init(
		svr.ctx, scheme, host, portN, sink, cfg,
	); err != nil {
		return nil, err
	}

	if err := client.Start(inter); err != nil {
		return nil, err
	} else {
		svr.instance.Store(client)
		if err := svr.connectReporter(); err != nil {
			svr.instance.Store(nil)
			return nil, err
		} else {
			return client, nil
		}
	}
}
