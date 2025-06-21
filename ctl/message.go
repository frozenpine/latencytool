package ctl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/frozenpine/latency4go"
	"github.com/valyala/bytebufferpool"
)

//go:generate stringer -type messageType -linecomment
type messageType uint8

const (
	MsgUnknown   messageType = iota // Unknown
	MsgCommand                      // Command
	MsgResult                       // Result
	MsgBroadCast                    // BroadCast
)

var (
	ErrInvalidMsgType = errors.New("invalid msg type")
	ErrInvalidMsgData = errors.New("invalid msg data")
)

func getData[T Command | Result | latency4go.State](data []byte) (*T, error) {
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

func (m *Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		MsgID   uint64
		MsgType messageType
		Data    []byte
	}{
		MsgID:   m.msgID,
		MsgType: m.msgType,
		Data:    m.data,
	})
}

func (m *Message) UnmarshalJSON(v []byte) error {
	var d struct {
		MsgID   uint64
		MsgType messageType
		Data    []byte
	}

	if err := json.Unmarshal(v, &d); err != nil {
		return err
	}

	m.msgID = d.MsgID
	m.msgType = d.MsgType
	m.data = d.Data

	return nil
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

func (m *Message) GetState() (*latency4go.State, error) {
	if m == nil {
		return nil, ErrInvalidMsgType
	}

	if m.msgType != MsgBroadCast && (m.msgType == MsgResult && m.msgID > 1) {
		return nil, fmt.Errorf("%w: not a state msg", ErrInvalidMsgType)
	}

	return getData[latency4go.State](m.data)
}

func (m *Message) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("Message{MsgID:")
	buff.WriteString(strconv.FormatUint(m.msgID, 10))
	buff.WriteString(" MessageType:")
	buff.WriteString(m.msgType.String())
	buff.WriteString(" Data:")
	buff.WriteString(string(m.data))
	buff.WriteString("}")

	return buff.String()
}

type Command struct {
	Name   string `json:"name"`
	KwArgs map[string]string
}

type Result struct {
	Rtn     int
	Message string
	CmdName string
	Values  map[string]any
}

func (r *Result) UnmarshalJSON(v []byte) error {
	data := make(map[string]json.RawMessage)

	if err := json.Unmarshal(v, &data); err != nil {
		return err
	}

	if err := json.Unmarshal(data["Rtn"], &r.Rtn); err != nil {
		return err
	}

	if err := json.Unmarshal(data["Message"], &r.Message); err != nil {
		return err
	}

	if err := json.Unmarshal(data["CmdName"], &r.CmdName); err != nil {
		return err
	}

	values := make(map[string]json.RawMessage)
	r.Values = make(map[string]any)
	if err := json.Unmarshal(data["Values"], &values); err != nil {
		return nil
	}

	for k, v := range values {
		r.Values[k] = v
	}

	return nil
}
