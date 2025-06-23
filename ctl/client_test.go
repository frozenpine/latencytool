package ctl

import "testing"

func TestSSHConn(t *testing.T) {
	conn := "abc:test@127.0.0.1?127.0.0.1:45678"

	match := sshConnPattern.FindStringSubmatch(conn)

	t.Log(match)
}
