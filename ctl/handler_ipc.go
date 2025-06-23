package ctl

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	ipc "github.com/james-barrow/golang-ipc"
)

type CtlIpcHandler struct {
	ctlBaseHandler

	server     *ipc.Server
	svrRunning atomic.Bool
}

func (ipcHdl *CtlIpcHandler) Write(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	err = ipcHdl.server.Write(1, data)
	if err != nil {
		switch ipcHdl.server.StatusCode() {
		case ipc.NotConnected, ipc.Listening:
			// filter out no client connection
			return nil
		default:
			return err
		}
	} else {
		return nil
	}
}

func (ipcHdl *CtlIpcHandler) Start() {
	ipcHdl.baseStart()

	ipcHdl.addConn("ipc client", ipcHdl)

	go func() {
		for ipcHdl.svrRunning.Load() {
			ipcMsg, err := ipcHdl.server.Read()

			if err != nil {
				slog.Error(
					"read from ipc failed",
					slog.Any("error", err),
				)

				if ipcHdl.server.StatusCode() == ipc.Closed {
					break
				}

				continue
			}

			if ipcMsg.MsgType > 0 {
				var msg Message
				if err := json.Unmarshal(ipcMsg.Data, &msg); err != nil {
					slog.Error(
						"unmarshal ipc message failed",
						slog.Any("error", err),
					)
				} else {
					ipcHdl.hdlCommandCache.Store(msg.msgID, ipcHdl)
					select {
					case ipcHdl.hdlCommands <- &msg:
					case <-time.After(time.Second * 5):
						slog.Warn("send message from IPC to ctl server timeout")
					}
				}
			} else {
				slog.Debug("ipc message", slog.Any("ipc_msg", ipcMsg))
			}
		}
	}()
}

func (ipcHdl *CtlIpcHandler) Release() {
	ipcHdl.server.Close()
	ipcHdl.svrRunning.Store(false)

	ipcHdl.ctlBaseHandler.baseRelease()
}

func NewIpcCtlHandler(name string) (*CtlIpcHandler, error) {
	svr, err := ipc.StartServer(name, nil)
	if err != nil {
		return nil, err
	}

	hdl := CtlIpcHandler{
		server: svr,
	}
	hdl.hdlName = fmt.Sprint("ctl_ipc_", name)
	hdl.connName = fmt.Sprint("ipc://", name)
	hdl.svrRunning.Store(true)

	return &hdl, nil
}
