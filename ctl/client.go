package ctl

import (
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/frozenpine/msgqueue/channel"
	"github.com/frozenpine/msgqueue/core"
	ipc "github.com/james-barrow/golang-ipc"
)

type CtlClient interface {
	core.Consumer[*Message]

	Start()
	Command(*Command) error
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

type CtlIPCClient struct {
	ctlBaseClient

	ipcClient *ipc.Client
}

func (client *CtlIPCClient) Start() {
	go func() {
		for {
			ipcMsg, err := client.ipcClient.Read()

			if err != nil {
				if client.ipcClient.StatusCode() == ipc.Closed {
					slog.Info(
						"ipc client closed",
						slog.Any("error", err),
						slog.String("status", client.ipcClient.Status()),
					)

					break
				}

				slog.Error(
					"read ipc message failed",
					slog.Any("error", err),
				)
				continue
			}

			if ipcMsg.MsgType > 0 {
				var msg Message
				if err = json.Unmarshal(ipcMsg.Data, &msg); err != nil {
					slog.Error(
						"unmarshal message failed",
						slog.Any("error", err),
						slog.Any("ipc_msg", ipcMsg),
					)
				} else if err = client.MemoChannel.Publish(
					&msg, time.Second*5,
				); err != nil {
					slog.Error(
						"publish message failed",
						slog.Any("error", err),
						slog.String("msg", msg.String()),
					)
				}
			} else {
				slog.Debug(
					"ipc message",
					slog.Any("ipc_msg", ipcMsg),
				)
			}
		}

		slog.Info("ipc channel closed")
	}()
}

func (client *CtlIPCClient) Command(cmd *Command) error {
	msg, err := client.createCmdMessage(cmd)
	if err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return client.ipcClient.Write(1, data)
}

func NewCtlIPCClient(conn string) (*CtlIPCClient, error) {
	client, err := ipc.StartClient(conn, nil)

	if err != nil {
		return nil, err
	}

	instance := &CtlIPCClient{
		ctlBaseClient: ctlBaseClient{
			name: conn,
		},

		ipcClient: client,
	}

	return instance, nil
}
