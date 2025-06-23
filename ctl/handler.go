package ctl

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
)

type Handler interface {
	core.Upstream[*Message]
	core.Producer[*Message]

	Start()
	Commands() <-chan *Message

	ConnName() string
	ConnCount() int
}

type messageWriter interface {
	Write(*Message) error
}

type ctlBaseHandler struct {
	channel.MemoChannel[*Message]

	hdlName         string
	connName        string
	hdlCommands     chan *Message
	hdlConnCount    atomic.Int32
	hdlConnections  sync.Map
	hdlCommandCache sync.Map
}

func (hdl *ctlBaseHandler) Name() string {
	return hdl.hdlName
}

func (hdl *ctlBaseHandler) ConnName() string {
	return hdl.connName
}

func (hdl *ctlBaseHandler) addConn(name string, wr messageWriter) {
	if _, exist := hdl.hdlConnections.LoadOrStore(name, wr); exist {
		slog.Error(
			"connection writer already exist",
			slog.String("name", name),
		)
	} else {
		hdl.hdlConnCount.Add(1)
	}
}

func (hdl *ctlBaseHandler) delConn(name any) {
	if _, exist := hdl.hdlConnections.LoadAndDelete(name); exist {
		hdl.hdlConnCount.Add(-1)
	} else {
		slog.Error(
			"connection writer not exist",
			slog.Any("name", name),
		)
	}
}

func (hdl *ctlBaseHandler) ConnCount() int {
	return int(hdl.hdlConnCount.Load())
}

func (hdl *ctlBaseHandler) baseStart() {
	hdl.hdlCommands = make(chan *Message, 10)

	go hdl.dispatchResults()
}

func (hdl *ctlBaseHandler) dispatchResults() {
	_, results := hdl.MemoChannel.Subscribe("ipc dispatcher", core.Quick)

	for msg := range results {
		switch msg.GetType() {
		case MsgBroadCast:
			hdl.hdlConnections.Range(func(key, value any) bool {
				wr, ok := value.(messageWriter)

				if !ok {
					slog.Error(
						"invalid connection writer",
						slog.Any("identity", key),
					)

					hdl.delConn(key)
				}

				err := wr.Write(msg)

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
				wr, ok := value.(messageWriter)

				if !ok {
					slog.Error(
						"invalid command writer",
						slog.Any("msg", msg),
					)
				}

				err := wr.Write(msg)

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

func (hdl *ctlBaseHandler) baseRelease() {
	close(hdl.hdlCommands)

	hdl.MemoChannel.Release()
}
