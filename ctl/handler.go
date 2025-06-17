package ctl

import (
	"fmt"

	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
	ipc "github.com/james-barrow/golang-ipc"
)

type Handler interface {
	core.Upstream[*Message]
	core.Producer[*Message]

	Commands() <-chan *Message
}

type ctlBaseHandler struct {
	channel.Channel[*Message]

	name     string
	commands chan *Message
}

func (hdl *ctlBaseHandler) Name() string {
	return hdl.name
}

func (hdl *ctlBaseHandler) Commands() <-chan *Message {
	return hdl.commands
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
