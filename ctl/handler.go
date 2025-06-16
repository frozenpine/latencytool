package ctl

import (
	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
)

type Handler interface {
	core.Upstream[Message]

	Release()
}

type HandlerType uint8

const (
	IPCHandler HandlerType = iota
	HttpHandler
)

type CtlHandler[T HandlerType] struct {
	channel.Channel[Message]
}
