package ctl

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type CtlSshTcpClient struct {
	*CtlTcpClient
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

	readDone := make(chan struct{})
	writeDone := make(chan struct{})

	go func() {
		defer close(readDone)

		if _, err := io.Copy(conn, pipe); err != nil {
			slog.Error(
				"forward ssh tunnerl to local conn failed",
				slog.Any("error", err),
			)
		}
	}()

	go func() {
		defer close(writeDone)

		if _, err := io.Copy(pipe, conn); err != nil {
			slog.Error(
				"forward local conn to ssh tunnerl failed",
				slog.Any("error", err),
			)
		}
	}()

	select {
	case <-readDone:
	case <-writeDone:
	}

	slog.Error("forwarding pipeline broken, exit forwarding")
}

var (
	sshConnPattern = regexp.MustCompile(
		`(?P<user>[^:]+):(?P<pass>[^@]*)@(?P<ssh>[^?]+)\?(?P<conn>.+)`,
	)

	userIdx = sshConnPattern.SubexpIndex("user")
	passIdx = sshConnPattern.SubexpIndex("pass")
	sshIdx  = sshConnPattern.SubexpIndex("ssh")
	connIdx = sshConnPattern.SubexpIndex("conn")
)

func NewCtlSshTcpClient(conn string) (*CtlSshTcpClient, error) {
	match := sshConnPattern.FindStringSubmatch(conn)

	if len(match) < 1 {
		return nil, errors.New("invalid ssh forward conn")
	}

	var (
		sshHost = match[sshIdx]
		sshUser = match[userIdx]
		sshPass = match[passIdx]
		sshAuth []ssh.AuthMethod
	)

	if !strings.Contains(sshHost, ":") {
		sshHost = sshHost + ":22"
	}

	if sshPass != "" && !strings.HasPrefix(sshPass, "key=") {
		slog.Info(
			"connect ssh with user/pass",
			slog.String("user", sshUser),
			slog.String("pass", strings.Map(func(r rune) rune {
				return '*'
			}, sshPass)),
		)

		sshAuth = append(sshAuth, ssh.Password(sshPass))
	} else {
		var keyFile string

		if sshPass == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			keyFile = filepath.Join(userHome, ".ssh", "id_rsa")
		} else {
			keyFile = strings.TrimPrefix(match[passIdx], "key=")
		}

		slog.Info(
			"connect ssh with user/key",
			slog.String("user", sshUser),
			slog.String("key", keyFile),
		)

		key, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}

		sign, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}

		sshAuth = append(sshAuth, ssh.PublicKeys(sign))
	}

	sshClient, err := ssh.Dial("tcp", sshHost, &ssh.ClientConfig{
		User:            sshUser,
		Auth:            sshAuth,
		Timeout:         time.Second * 15,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return nil, err
	}

	pipe, err := sshClient.Dial("tcp", match[connIdx])
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
			slog.Any("lsnr", lsnr.Addr()),
		)
	}

	go forward(sshClient, lsnr, pipe)

	inner, err := NewCtlTcpClient(lsnr.Addr().String())
	if err != nil {
		lsnr.Close()
		return nil, err
	}

	client := CtlSshTcpClient{
		CtlTcpClient: inner,
	}

	return &client, nil
}
