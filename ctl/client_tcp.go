package ctl

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"time"
)

type CtlTcpClient struct {
	ctlBaseClient

	conn net.Conn
}

func (c *CtlTcpClient) recv() {
	defer c.conn.Close()

	rd := bufio.NewScanner(c.conn)

	for rd.Scan() {
		var msg Message

		if err := json.Unmarshal(rd.Bytes(), &msg); err != nil {
			slog.Error(
				"unmarshal tcp message failed",
				slog.Any("error", err),
			)
			continue
		}

		if err := c.MemoChannel.Publish(&msg, time.Second*5); err != nil {
			slog.Error(
				"publish message failed",
				slog.Any("error", err),
				slog.String("msg", msg.String()),
			)
		}
	}
}

func (c *CtlTcpClient) Start() {
	go c.recv()
}

func (c *CtlTcpClient) Command(cmd *Command) error {
	msg, err := c.createCmdMessage(cmd)
	if err != nil {
		return err
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(data)
	return err
}

func (c *CtlTcpClient) Release() {
	c.conn.Close()

	c.ctlBaseClient.Release()
}

func NewCtlTcpClient(conn string) (*CtlTcpClient, error) {
	c, err := net.Dial("tcp4", conn)
	if err != nil {
		return nil, err
	}

	client := CtlTcpClient{
		ctlBaseClient: ctlBaseClient{
			name: conn,
		},
		conn: c,
	}

	return &client, nil
}
