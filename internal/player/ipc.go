package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"podmutt/internal/feedback"
	"sync"
	"time"
)

var ErrDisconnected = feedback.New(feedback.PlayerFailure, "mpv: IPC disconnected")
var ErrEventOverflow = feedback.New(feedback.PlayerFailure, "mpv: event consumer fell behind")

type Event struct {
	PlaylistEntryID *int64          `json:"playlist_entry_id"`
	Name            string          `json:"event"`
	Property        string          `json:"name"`
	Data            json.RawMessage `json:"data"`
	Reason          string          `json:"reason"`
	Error           string          `json:"error"`
	FileError       json.RawMessage `json:"file_error"`
}

type reply struct {
	data json.RawMessage
	err  error
}

type ipc struct {
	conn      net.Conn
	timeout   time.Duration
	writeGate chan struct{}
	mu        sync.Mutex
	next      uint64
	pending   map[uint64]chan reply
	err       error
	events    chan Event
	done      chan struct{}
}

func newIPC(conn net.Conn, timeout time.Duration) *ipc {
	c := &ipc{conn: conn, timeout: timeout, pending: make(map[uint64]chan reply),
		events: make(chan Event, 128), done: make(chan struct{}), writeGate: make(chan struct{}, 1)}
	go c.read()
	return c
}

func (c *ipc) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	c.err = err
	for id, response := range c.pending {
		response <- reply{err: err}
		delete(c.pending, id)
	}
	c.conn.Close()
}

func (c *ipc) read() {
	defer close(c.done)
	defer close(c.events)
	defer c.fail(ErrDisconnected)
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var frame struct {
			Event
			RequestID uint64 `json:"request_id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			c.fail(&feedback.Fault{Code: feedback.PlayerFailure, Safe: "mpv: invalid IPC message", Cause: err})
			return
		}
		if frame.RequestID != 0 {
			c.mu.Lock()
			if response, ok := c.pending[frame.RequestID]; ok {
				result := reply{data: frame.Data}
				if frame.Error != "success" {
					result.err = &feedback.Fault{Code: feedback.PlayerFailure, Safe: "mpv: command failed", Cause: errors.New(frame.Error)}
				}
				response <- result
				delete(c.pending, frame.RequestID)
			}
			c.mu.Unlock()
		} else if frame.Name != "" {
			select {
			case c.events <- frame.Event:
			default:
				c.fail(ErrEventOverflow)
				return
			}
		}
	}
}

func (c *ipc) command(ctx context.Context, command any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	c.next++
	id := c.next
	response := make(chan reply, 1)
	c.pending[id] = response
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	select {
	case c.writeGate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-c.writeGate
		return nil, err
	}
	deadline, _ := ctx.Deadline()
	c.conn.SetWriteDeadline(deadline)
	err := json.NewEncoder(c.conn).Encode(map[string]any{"command": command, "request_id": id})
	<-c.writeGate
	if err != nil {
		c.fail(ErrDisconnected)
		return nil, ErrDisconnected
	}
	select {
	case result := <-response:
		return result.data, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *ipc) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
