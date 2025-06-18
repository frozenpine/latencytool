package ctl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
	ipc "github.com/james-barrow/golang-ipc"
)

type Handler interface {
	Init(context.Context, string)
	Name() string
	Start()
	Stop()
	Join() error
	Publish(*Message, time.Duration) error
	Commands() <-chan *Message
	Results() core.Upstream[*Message]
}

type ctlBaseHandler struct {
	results channel.MemoChannel[*Message]

	hdlName         string
	hdlCommands     chan *Message
	hdlConnections  sync.Map
	hdlCommandCache sync.Map
}

func (hdl *ctlBaseHandler) Name() string {
	return hdl.hdlName
}

func (hdl *ctlBaseHandler) Init(ctx context.Context, name string) {
	hdl.results.Init(ctx, name, nil)
}

func (hdl *ctlBaseHandler) Publish(msg *Message, timeout time.Duration) error {
	return hdl.results.Publish(msg, timeout)
}

func (hdl *ctlBaseHandler) Results() core.Upstream[*Message] {
	return &hdl.results
}

func (hdl *ctlBaseHandler) baseStart() {
	hdl.hdlCommands = make(chan *Message, 10)

	go hdl.dispatchResults()
}

func (hdl *ctlBaseHandler) dispatchResults() {
	_, results := hdl.results.Subscribe("ipc dispatcher", core.Quick)

	for msg := range results {
		data, err := json.Marshal(msg)

		if err != nil {
			slog.Error(
				"marshal message failed",
				slog.Any("error", err),
				slog.Any("msg", msg),
			)

			continue
		}

		switch msg.GetType() {
		case MsgBroadCast:
			hdl.hdlConnections.Range(func(key, value any) bool {
				wr, ok := value.(io.Writer)

				if !ok {
					slog.Error(
						"invalid connection writer",
						slog.Any("identity", key),
					)
					hdl.hdlConnections.Delete(key)
				}

				_, err := wr.Write(data)

				if err != nil {
					slog.Error(
						"write broadcast msg failed",
						slog.Any("error", err),
						slog.Any("identity", key),
					)
				}

				return true
			})
		case MsgResult:
			if value, loaded := hdl.hdlCommandCache.LoadAndDelete(
				msg.msgID,
			); loaded {
				wr, ok := value.(io.Writer)

				if !ok {
					slog.Error(
						"invalid command writer",
						slog.Any("msg", msg),
					)
				}

				_, err := wr.Write(data)

				if err != nil {
					slog.Error(
						"write command result failed",
						slog.Any("error", err),
						slog.Any("msg", msg),
					)
				}
			} else {
				slog.Error(
					"no command writer found",
					slog.Any("msg", msg),
				)
			}
		default:
			slog.Error(
				"invalid rtn msg type",
				slog.Any("msg", msg),
			)
		}
	}

	slog.Info(
		"result channel closed",
		slog.String("handler", hdl.hdlName),
	)
}

func (hdl *ctlBaseHandler) Commands() <-chan *Message {
	return hdl.hdlCommands
}

func (hdl *ctlBaseHandler) Join() error {
	hdl.results.Join()

	// TODO
	return nil
}

func (hdl *ctlBaseHandler) baseRelease() {
	close(hdl.hdlCommands)

	hdl.results.Release()
}

type CtlIPCHandler struct {
	ctlBaseHandler

	server     *ipc.Server
	svrRunning atomic.Bool
}

func (ipcHdl *CtlIPCHandler) Write(data []byte) (int, error) {
	err := ipcHdl.server.Write(1, data)
	if err != nil {
		switch ipcHdl.server.StatusCode() {
		case ipc.NotConnected, ipc.Listening:
			return len(data), nil
		default:
			return 0, err
		}
	} else {
		return len(data), nil
	}
}

func (ipcHdl *CtlIPCHandler) Start() {
	ipcHdl.baseStart()

	ipcHdl.hdlConnections.Store("ipc client", ipcHdl)

	go func() {
		for ipcHdl.svrRunning.Load() {
			ipcMsg, err := ipcHdl.server.Read()

			if err != nil {
				slog.Error(
					"read from ipc failed",
					slog.Any("error", err),
				)

				if ipcHdl.server.StatusCode() == ipc.Closed {
					break
				}

				continue
			}

			var msg Message
			if err := json.Unmarshal(ipcMsg.Data, &msg); err != nil {
				slog.Error(
					"unmarshal ipc message failed",
					slog.Any("error", err),
				)
			} else {
				ipcHdl.hdlCommandCache.Store(msg.msgID, ipcHdl)
				select {
				case ipcHdl.hdlCommands <- &msg:
				case <-time.After(time.Second * 5):
					slog.Warn("send message from IPC to ctl server timeout")
				}
			}
		}
	}()
}

func (ipcHdl *CtlIPCHandler) Stop() {
	ipcHdl.server.Close()
	ipcHdl.svrRunning.Store(false)

	ipcHdl.ctlBaseHandler.baseRelease()
}

func NewIpcCtlHandler(name string) (*CtlIPCHandler, error) {
	svr, err := ipc.StartServer(name, nil)
	if err != nil {
		return nil, err
	}

	hdl := CtlIPCHandler{
		server: svr,
	}
	hdl.hdlName = fmt.Sprintf("ctl_ipc_%s", name)
	hdl.svrRunning.Store(true)

	return &hdl, nil
}
