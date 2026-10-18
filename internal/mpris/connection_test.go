package mpris

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalAddressPolicy(t *testing.T) {
	for _, address := range []string{"", "autolaunch:", "tcp:host=example.test", "unix:path=relative", "unix:path=%00", "unix:path=%zz", "unix:bad", "unix:path=/a,abstract=b"} {
		if _, err := unixAddresses(address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
	addresses, err := unixAddresses("unix:path=/tmp/a%2Cb,guid=ignored;unix:abstract=podmutt+bus")
	if err != nil || len(addresses) != 2 || addresses[0] != "/tmp/a,b" || addresses[1] != "\x00podmutt+bus" {
		t.Fatalf("addresses: %q %v", addresses, err)
	}
}

func TestHungAuthenticationIsBoundedAndErrorsAreRedacted(t *testing.T) {
	dir, err := os.MkdirTemp("", "pmb-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "bus")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	startup, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := connect(startup, context.Background(), "unix:path="+path); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	select {
	case conn := <-accepted:
		conn.Close()
	case <-time.After(time.Second):
		t.Fatal("no connection accepted")
	}
	if _, err := connect(context.Background(), context.Background(), "unix:path=/missing/secret-address"); err == nil || strings.Contains(err.Error(), "secret-address") {
		t.Fatal("unavailable bus error disclosed address")
	}
}
