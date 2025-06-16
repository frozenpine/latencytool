package ctl

import (
	"encoding/json"
	"errors"
	"fmt"
)

type messageType uint8

const (
	MsgUnknown messageType = iota
	MsgCommand
	MsgResult
	MsgBroadCast
)

var (
	ErrInvalidMsgType = errors.New("invalid msg type")
	ErrInvalidMsgData = errors.New("invalid msg data")
)

func getData[T Command | Result | BroadCast](data []byte) (*T, error) {
	var v T

	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("%w: %+v", ErrInvalidMsgData, err)
	}

	return &v, nil
}

type Message struct {
	// 每会话唯一单调递增，用于标识Commnd/Result对
	// 0 值保留用于广播消息
	msgID   uint64
	msgType messageType
	data    []byte
}

func (m *Message) GetType() messageType {
	if m == nil {
		return MsgUnknown
	}

	return m.msgType
}

func (m *Message) GetID() uint64 {
	return m.msgID
}

func (m *Message) GetCommand() (*Command, error) {
	if m == nil {
		return nil, ErrInvalidMsgType
	}

	if m.msgType != MsgCommand {
		return nil, fmt.Errorf("%w: not a command msg", ErrInvalidMsgType)
	}

	return getData[Command](m.data)
}

func (m *Message) GetResult() (result *Result, err error) {
	if m == nil {
		return nil, ErrInvalidMsgType
	}

	if m.msgType != MsgResult {
		return nil, fmt.Errorf("%w: not a result msg", ErrInvalidMsgType)
	}

	return getData[Result](m.data)
}

func (m *Message) GetBroadCast() (*BroadCast, error) {
	if m == nil {
		return nil, ErrInvalidMsgType
	}

	if m.msgType != MsgResult {
		return nil, fmt.Errorf("%w: not a broadcast msg", ErrInvalidMsgType)
	}

	return getData[BroadCast](m.data)
}

type cmd string

const (
	Cmdconfig   cmd = "config"
	CmdReload   cmd = "reload"
	CmdRestart  cmd = "restart"
	CmdStop     cmd = "stop"
	CmdShow     cmd = "show"
	CmdPipeline cmd = "pipeline"
)

type Command struct {
	Name   cmd `json:"name"`
	KwArgs map[string]string
}

func (cmd *Command) Chain() ([]*Command, error) {
	// TODO
	return nil, nil
}

type rtnCode int

type values map[string]json.RawMessage

type Result struct {
	Rtn     rtnCode
	Message string
	CmdName cmd
	Values  values
}

type BroadCast values
