package ctl

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
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

	go func() {
		subId, notify := c.Subscribe(name, core.Quick)

		slog.Info(
			"message loop get new subscribe",
			slog.Any("name", name),
			slog.String("sub_id", subId.String()),
		)

		defer func() {
			c.UnSubscribe(subId)

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
			var state *latency4go.State

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

				if result.Rtn != 0 {
					slog.Error(
						"command execution failed",
						slog.String("cmd", result.CmdName),
						slog.String("error_msg", result.Message),
					)
					continue
				}

				// 第一笔默认发送 `state` 命令不会调
				if handleResult != nil && msg.msgID > 1 {
					if err := handleResult(result); err != nil {
						slog.Error(
							"handle result failed",
							slog.Any("error", err),
							slog.String("name", name),
						)
					}
				}

				switch result.CmdName {
				case "state":
					var rtn latency4go.State

					if err := json.Unmarshal(
						result.Values["state"].(json.RawMessage), &rtn,
					); err != nil {
						slog.Error(
							"unmarshal state failed",
							slog.Any("error", err),
						)
						continue
					}

					state = &rtn
				default:
					slog.Log(
						context.Background(), slog.LevelDebug-1,
						"command result notified",
						slog.String("name", name),
						slog.Any("result", result),
					)
					continue
				}
			case MsgBroadCast:
				brd, err := msg.GetState()
				if err != nil {
					slog.Error(
						"get state message failed",
						slog.Any("error", err),
					)
					continue
				}
				state = brd
			default:
				slog.Warn(
					"unsupported return msg from ctl server",
					slog.Any("result", msg),
				)
				continue
			}

			if state != nil {
				slog.Log(
					context.Background(), slog.LevelDebug-1,
					"latency state notified",
					slog.String("name", name),
					slog.Time("timestamp", state.Timestamp),
					slog.Any("config", state.Config),
				)

				if handleState == nil {
					continue
				}

				if err := handleState(state); err != nil {
					slog.Error(
						"message loop handle state failed",
						slog.Any("error", err),
						slog.String("name", name),
					)
				}
			} else {
				slog.Error("state is empty")
			}
		}

		slog.Info("ctl client message loop exit")
	}()

	return nil
}
