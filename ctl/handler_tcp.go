package ctl

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"
)

type CtlTcpHandler struct {
	ctlBaseHandler

	listen net.Listener
}

type tcpMsgWriter struct {
	hdl      *CtlTcpHandler
	conn     net.Conn
	identity string
	mask     uint64
}

func (wr *tcpMsgWriter) Write(msg *Message) error {
	if msg.msgID > 0 {
		msg.msgID ^= wr.mask
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buff := make([]byte, len(data)+1)
	buff[copy(buff, data)] = '\n'

	if _, err = wr.conn.Write(buff); err != nil {
		wr.hdl.delConn(wr.identity)
		return err
	}

	return nil
}

func (tcpHdl *CtlTcpHandler) handleConn(conn net.Conn) {
	remote := conn.RemoteAddr().(*net.TCPAddr)
	remoteIdt := remote.String()
	remoteIP := remote.IP.To4()
	remotePort := uint64(remote.Port)

	mask := uint64(binary.LittleEndian.Uint32(remoteIP))<<32 |
		remotePort<<16

	slog.Info(
		"tcp ctl client connected",
		slog.String("remote", remoteIdt),
	)

	rd := bufio.NewScanner(conn)
	wr := &tcpMsgWriter{
		hdl:      tcpHdl,
		conn:     conn,
		identity: remoteIdt,
		mask:     mask,
	}

	tcpHdl.addConn(remoteIdt, wr)

	defer func() {
		tcpHdl.delConn(remoteIdt)

		conn.Close()
	}()

	for rd.Scan() {
		var msg Message
		if err := json.Unmarshal(rd.Bytes(), &msg); err != nil {
			slog.Error(
				"unmarshal tcp message failed",
				slog.Any("error", err),
			)
		} else {
			if msg.msgID > 0 {
				msg.msgID = (msg.msgID & 0x00000000FFFFFFFF) | mask
			}

			tcpHdl.hdlCommandCache.Store(msg.msgID, wr)
			select {
			case tcpHdl.hdlCommands <- &msg:
			case <-time.After(time.Second * 5):
				slog.Warn("send message from IPC to ctl server timeout")
			}
		}
	}

	slog.Info(
		"tcp ctl client read exit",
		slog.String("remote", remoteIdt),
	)
}

func (tcpHdl *CtlTcpHandler) Start() {
	tcpHdl.baseStart()

	go func() {
		for {
			conn, err := tcpHdl.listen.Accept()
			if err != nil {
				slog.Error(
					"accept tcp client failed",
					slog.Any("error", err),
				)
				return
			}

			go tcpHdl.handleConn(conn)
		}
	}()
}

func (tcpHdl *CtlTcpHandler) Release() {
	if err := tcpHdl.listen.Close(); err != nil {
		slog.Error(
			"close tcp listener failed",
			slog.Any("error", err),
		)
	}

	tcpHdl.ctlBaseHandler.baseRelease()
}

func NewCtlTcpHandler(conn string) (*CtlTcpHandler, error) {
	listen, err := net.Listen("tcp4", conn)
	if err != nil {
		return nil, err
	}

	hdl := CtlTcpHandler{
		listen: listen,
	}
	hdl.hdlName = fmt.Sprint("ctl_tcp_", conn)
	hdl.connName = fmt.Sprint("tcp://", conn)

	return &hdl, nil
}
