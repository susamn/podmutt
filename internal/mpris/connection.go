package mpris

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

var ErrUnavailable = errors.New("MPRIS unavailable (session D-Bus missing, disconnected, or name in use)")

const startupTimeout = 2 * time.Second

func unixAddresses(address string) ([]string, error) {
	var result []string
	for _, candidate := range strings.Split(address, ";") {
		transport, fields, ok := strings.Cut(candidate, ":")
		if !ok || transport != "unix" {
			continue
		}
		var path, abstract string
		for _, field := range strings.Split(fields, ",") {
			key, raw, ok := strings.Cut(field, "=")
			if !ok {
				return nil, ErrUnavailable
			}
			value, err := url.PathUnescape(raw)
			if err != nil || strings.ContainsRune(value, 0) {
				return nil, ErrUnavailable
			}
			switch key {
			case "path":
				path = value
			case "abstract":
				abstract = value
			}
		}
		if path != "" && abstract != "" {
			return nil, ErrUnavailable
		}
		if filepath.IsAbs(path) {
			result = append(result, path)
		} else if abstract != "" {
			result = append(result, "\x00"+abstract)
		}
	}
	if len(result) == 0 {
		return nil, ErrUnavailable
	}
	return result, nil
}

// connect bounds dialing, authentication and Hello, including a hung peer.
func connect(startup, lifetime context.Context, address string) (*dbus.Conn, error) {
	targets, err := unixAddresses(address)
	if err != nil {
		return nil, err
	}
	var raw net.Conn
	for _, target := range targets {
		raw, err = (&net.Dialer{}).DialContext(startup, "unix", target)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	conn, err := dbus.NewConn(raw, dbus.WithContext(lifetime))
	if err != nil {
		raw.Close()
		return nil, ErrUnavailable
	}
	stop := context.AfterFunc(startup, func() { conn.Close() })
	defer stop()
	deadline, _ := startup.Deadline()
	raw.SetDeadline(deadline)
	if err := conn.Auth([]dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Geteuid()))}); err != nil {
		conn.Close()
		return nil, ErrUnavailable
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, ErrUnavailable
	}
	if err := raw.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, ErrUnavailable
	}
	return conn, nil
}
