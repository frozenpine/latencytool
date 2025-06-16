package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	clients   sync.Map
	broadcast channel.MemoChannel[Message]
}

func NewCtlServer(
	ctx context.Context, instance *atomic.Pointer[latency4go.LatencyClient],
) (svr *CtlServer, err error) {
	if instance == nil {
		return nil, fmt.Errorf(
			"%w: no latency client instance", ErrCtlServerArgs,
		)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	svr = &CtlServer{}

	svr.initOnce.Do(func() {
		svr.ctx, svr.cancel = context.WithCancel(ctx)

		svr.instance = instance

		svr.broadcast.Init(svr.ctx, "ctl", nil)

		if err = svr.instance.Load().AddReporter(
			"ctl",
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
					if err = svr.broadcast.Publish(Message{
						msgType: MsgBroadCast,
						data:    data,
					}, time.Second*5); err != nil {
						slog.Error("ctl reporter publish priorities timeout")
					}
				}

				return nil
			},
		); err != nil {
			err = errors.Join(ErrInitCtlServer, err)

			return
		}
	})

	return
}

func (svr *CtlServer) Start() error {
	svr.startOnce.Do(func() {
		// TODO
		go svr.runForever()
	})

	return nil
}

func (svr *CtlServer) Stop() {
	svr.stopOnce.Do(func() {
		// TODO
	})
}

func (svr *CtlServer) Join() error {
	// TODO
	return nil
}

func (svr *CtlServer) accept() (Handler, error) {
	// TODO
	return nil, nil
}

func (svr *CtlServer) runForever() {
	defer svr.Stop()

	for {
		select {
		case <-svr.ctx.Done():
			return
		default:
			hdl, err := svr.accept()

			if err != nil {
				slog.Error(
					"ctl server accept client failed",
					slog.Any("error", err),
				)

				hdl.Release()
				continue
			}

			if err = svr.broadcast.PipelineDownStream(hdl); err != nil {
				slog.Error(
					"connect client's broad failed",
					slog.Any("error", err),
				)

				hdl.Release()
				continue
			}

			if !svr.clients.CompareAndSwap(hdl.Name(), nil, hdl) {
				slog.Error(
					"client already exits",
					slog.String("name", hdl.Name()),
				)

				hdl.Release()
			}
		}
	}
}
