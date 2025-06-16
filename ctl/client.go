package ctl

import (
	"github.com/frozenpine/msgqueue/channel"
)

type CtlClient[T any] struct {
	name    string
	results channel.Channel[Message]
}

func (c *CtlClient[T]) Name() string {
	return c.name
}

func (c *CtlClient[T]) Release() {}
