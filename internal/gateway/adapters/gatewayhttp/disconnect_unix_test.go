//go:build unix

package gatewayhttp

import (
	"net"
	"syscall"
	"testing"
	"time"
)

func TestPeerClosed(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	client, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	server, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	raw, err := server.(syscall.Conn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}

	if peerClosed(raw) {
		t.Fatal("open idle connection reported closed")
	}
	if _, err := client.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if peerClosed(raw) {
		t.Fatal("connection with unread data reported closed")
	}
	buf := make([]byte, 1)
	if n, _ := server.Read(buf); n != 1 {
		t.Fatal("peek consumed the pending byte")
	}

	_ = client.Close()
	deadline := time.Now().Add(2 * time.Second)
	for !peerClosed(raw) {
		if time.Now().After(deadline) {
			t.Fatal("closed connection never reported closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
