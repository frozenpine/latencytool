package ctl

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

type CtlSshTcpClient struct {
	*CtlTcpClient

	sshHost   string
	sshCfg    ssh.ClientConfig
	sshClient *ssh.Client
	lsnr      net.Listener
}

func forward(
	client *ssh.Client, lsnr net.Listener, pipe net.Conn,
) {
	defer func() {
		client.Close()
		pipe.Close()
	}()

	conn, err := lsnr.Accept()
	if err != nil {
		slog.Error(
			"accept local tcp client failed",
			slog.Any("error", err),
		)

		return
	}
	defer func() {
		conn.Close()
		lsnr.Close()
	}()

	wg := sync.WaitGroup{}

	wg.Add(2)
	go func() {
		defer wg.Done()

		if _, err := io.Copy(conn, pipe); err != nil {
			slog.Error(
				"forward ssh tunnerl to local conn failed",
				slog.Any("error", err),
			)
		}
	}()

	go func() {
		defer wg.Done()

		if _, err := io.Copy(pipe, conn); err != nil {
			slog.Error(
				"forward local conn to ssh tunnerl failed",
				slog.Any("error", err),
			)
		}
	}()

	wg.Wait()
}

func NewCtlSshTcpClient(conn string) (*CtlSshTcpClient, error) {
	var (
		sshHost string
		sshCfg  ssh.ClientConfig

		remoteConn string
	)

	sshClient, err := ssh.Dial("tcp", sshHost, &sshCfg)
	if err != nil {
		return nil, err
	}

	pipe, err := sshClient.Dial("tcp", remoteConn)
	if err != nil {
		return nil, err
	}

	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	lsnr, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	} else {
		slog.Info(
			"open local listenner for forwarding",
			slog.Any("lsnr", addr),
		)
	}

	go forward(sshClient, lsnr, pipe)

	inner, err := NewCtlTcpClient(fmt.Sprintf(
		"tcp://%s:%d", addr.IP.String(), addr.Port,
	))
	if err != nil {
		lsnr.Close()
		return nil, err
	}

	client := CtlSshTcpClient{
		CtlTcpClient: inner,
		sshHost:      sshHost,
		sshCfg:       sshCfg,
		sshClient:    sshClient,
		lsnr:         lsnr,
	}

	return &client, nil
}
