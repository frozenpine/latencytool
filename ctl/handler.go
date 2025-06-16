package ctl

import (
	"context"
	"fmt"
	"sync"

	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
	ipc "github.com/james-barrow/golang-ipc"
)

type Handler interface {
	core.Upstream[Message]
}

type ctlBaseHandler struct {
	channel.Channel[Message]

	name      string
	hdlCtx    context.Context
	hdlCancel context.CancelFunc
	initOnce  sync.Once
	stopOnce  sync.Once
}

func (hdl *ctlBaseHandler) Init(ctx context.Context) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	hdl.initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		hdl.hdlCtx, hdl.hdlCancel = context.WithCancel(ctx)

		hdl.Channel.Init(hdl.hdlCtx, hdl.name, nil)
	})

	return
}

func (hdl *ctlBaseHandler) Start() error {
	// TODO
	return nil
}

func (hdl *ctlBaseHandler) Stop() {
	hdl.stopOnce.Do(func() {
		// TODO

		hdl.hdlCancel()
	})
}

func (hdl *ctlBaseHandler) Join() {
	hdl.Channel.Join()

	// TODO
}

type CtlIPCHandler struct {
	ctlBaseHandler

	server *ipc.Server
}

func NewIpcCtlHandler(name string) (*CtlIPCHandler, error) {
	svr, err := ipc.StartServer(name, nil)
	if err != nil {
		return nil, err
	}

	hdl := CtlIPCHandler{
		server: svr,
	}
	hdl.name = fmt.Sprintf("ctl_ipc_%s", name)

	return &hdl, nil
}
