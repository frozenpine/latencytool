package ctl

import (
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	ipc "github.com/james-barrow/golang-ipc"
)

type CtlIpcClient struct {
	ctlBaseClient

	ipcClient *ipc.Client
	waitConn  chan struct{}
}

func (client *CtlIpcClient) Start() {
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

				switch client.ipcClient.StatusCode() {
				case ipc.Connected:
					close(client.waitConn)
				}
			}
		}

		slog.Info("ipc channel closed")
	}()

	if err := client.Command(&Command{
		Name: "info",
	}); err != nil {
		slog.Error("make initial info command failed", slog.Any("error", err))
	} else {
		slog.Info("initial info command sended")
	}
}

func (client *CtlIpcClient) Command(cmd *Command) error {
	<-client.waitConn

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

func (client *CtlIpcClient) Release() {
	client.ipcClient.Close()

	client.ctlBaseClient.Release()
}

func NewCtlIpcClient(conn string) (*CtlIpcClient, error) {
	waitChan := make(chan struct {
		c   *ipc.Client
		err error
	})

	go func() {
		client, err := ipc.StartClient(conn, nil)
		waitChan <- struct {
			c   *ipc.Client
			err error
		}{
			c:   client,
			err: err,
		}
	}()

	var client *ipc.Client
	select {
	case <-time.After(time.Second * 10):
		return nil, errors.New("connect ipc timeout")
	case r := <-waitChan:
		if r.err != nil {
			return nil, r.err
		} else {
			client = r.c
		}
	}

	instance := &CtlIpcClient{
		ctlBaseClient: ctlBaseClient{
			name: conn,
		},

		ipcClient: client,
		waitConn:  make(chan struct{}),
	}

	return instance, nil
}
