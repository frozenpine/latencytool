package ctl

import (
	"encoding/json"
	"sync/atomic"

	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
)

type CtlClient interface {
	core.Consumer[*Message]

	Start()
	Command(*Command) error
	GetCmdSeq() uint64
}

type ctlBaseClient struct {
	channel.MemoChannel[*Message]
	name   string
	cmdSeq atomic.Uint64
}

func (c *ctlBaseClient) Name() string {
	return c.name
}

func (c *ctlBaseClient) createCmdMessage(cmd *Command) (*Message, error) {
	cmdData, err := json.Marshal(cmd)

	if err != nil {
		return nil, err
	}

	return &Message{
		msgID:   c.cmdSeq.Add(1),
		msgType: MsgCommand,
		data:    cmdData,
	}, nil
}

func (c *ctlBaseClient) GetCmdSeq() uint64 {
	return c.cmdSeq.Load()
}
