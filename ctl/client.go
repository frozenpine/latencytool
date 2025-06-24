package ctl

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
	"github.com/gofrs/uuid"
)

type CtlClient interface {
	core.Consumer[*Message]

	Start()
	Command(cmd *Command) error
	GetCmdSeq() uint64
	MessageLoop(
		name string,
		preRun func() error,
		handleState func(*latency4go.State) error,
		handleResult func(*Result) error,
		postRun func() error,
	) error
}

type ctlBaseClient struct {
	channel.MemoChannel[*Message]
	name   string
	cmdSeq atomic.Uint64

	msgLoopSubs sync.Map
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

func (c *ctlBaseClient) closeLoop() {
	c.msgLoopSubs.Range(func(key, value any) bool {
		if sub, ok := key.(uuid.UUID); !ok {
			slog.Error(
				"invalid message loop sub id",
				slog.Any("key", key),
			)
		} else {
			c.UnSubscribe(sub)
		}

		return true
	})

	c.msgLoopSubs.Clear()
}

func (c *ctlBaseClient) MessageLoop(
	name string,
	preRun func() error,
	handleState func(*latency4go.State) error,
	handleResult func(*Result) error,
	postRun func() error,
) error {
	if preRun != nil {
		if err := preRun(); err != nil {
			return err
		}
	}

	if handleState == nil {
		handleState = LogState
	}

	if handleResult == nil {
		handleResult = LogResult
	}

	closeWait := make(chan struct{})

	go func() {
		subId, notify := c.Subscribe(name, core.Quick)

		slog.Info(
			"message loop get new subscribe",
			slog.Any("name", name),
			slog.String("sub_id", subId.String()),
		)

		c.msgLoopSubs.Store(subId, struct{}{})

		defer func() {
			defer func() {
				c.UnSubscribe(subId)
				close(closeWait)
			}()

			if postRun == nil {
				return
			}

			if err := postRun(); err != nil {
				slog.Error(
					"state loop post run failed",
					slog.Any("error", err),
				)
			}
		}()

		for msg := range notify {
			switch msg.GetType() {
			case MsgResult:
				result, err := msg.GetResult()

				if err != nil {
					slog.Error(
						"get result message failed",
						slog.Any("error", err),
					)
					continue
				}

				if handleResult != nil {
					if err = handleResult(result); err != nil {
						slog.Error(
							"message loop handle result failed",
							slog.Any("error", err),
							slog.String("name", name),
						)
					}
				} else if result.Rtn != 0 {
					slog.Error(
						"command execution failed",
						slog.String("cmd", result.CmdName),
						slog.String("error_msg", result.Message),
					)
				}
			case MsgBroadCast:
				state, err := msg.GetState()

				if err != nil {
					slog.Error(
						"get state message failed",
						slog.Any("error", err),
					)
					continue
				}

				if handleState == nil {
					slog.Log(
						context.Background(), slog.LevelDebug-1,
						"latency state notified",
						slog.String("name", name),
						slog.Time("timestamp", state.Timestamp),
						slog.Any("config", state.Config),
					)
				} else if err := handleState(state); err != nil {
					slog.Error(
						"message loop handle state failed",
						slog.Any("error", err),
						slog.String("name", name),
					)
				}
			default:
				slog.Warn(
					"unsupported return msg from ctl server",
					slog.Any("result", msg),
				)
			}
		}

		slog.Info("ctl client message loop exit")
	}()

	return nil
}
