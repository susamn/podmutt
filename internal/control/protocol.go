// Package control exposes foreground playback actions over a private Unix socket.
package control

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"podmutt/internal/feedback"
	"time"

	"podmutt/internal/playback"
	"podmutt/internal/store"
)

var ErrNotRunning = feedback.New(feedback.NotRunning, "podmutt: not running for this profile")

const timeout = 5 * time.Second
const maxRequest = 4096
const maxResponse = 1 << 20

type Actions interface {
	Play(context.Context) error
	Pause(context.Context) error
	Stop(context.Context) error
	Next(context.Context) error
	Snapshot(context.Context) (playback.State, error)
}

type Episode struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Position  float64 `json:"position_seconds"`
	Duration  int64   `json:"duration_seconds"`
	Completed bool    `json:"completed"`
}
type Status struct {
	Status  playback.Status `json:"status"`
	Current *Episode        `json:"current"`
	Queue   []Episode       `json:"queue"`
	Error   string          `json:"error,omitempty"`
}
type request struct {
	Command string `json:"command"`
}
type response struct {
	State *Status `json:"state,omitempty"`
	Error string  `json:"error,omitempty"`
}

func Valid(command string) bool {
	switch command {
	case "play", "pause", "stop", "next", "status":
		return true
	}
	return false
}

// Execute is the common dispatcher for local keyboard and remote CLI actions.
func Execute(ctx context.Context, actions Actions, command string) (Status, error) {
	var err error
	switch command {
	case "play":
		err = actions.Play(ctx)
	case "pause":
		err = actions.Pause(ctx)
	case "stop":
		err = actions.Stop(ctx)
	case "next":
		err = actions.Next(ctx)
	case "status":
	default:
		return Status{}, fmt.Errorf("unknown control command")
	}
	if err != nil {
		return Status{}, err
	}
	state, err := actions.Snapshot(ctx)
	if err != nil {
		return Status{}, err
	}
	result := Status{Status: state.Status, Error: state.Error, Queue: []Episode{}}
	if state.Current != nil {
		episode := brief(*state.Current)
		result.Current = &episode
	}
	for _, episode := range state.Queue {
		result.Queue = append(result.Queue, brief(episode))
	}
	return result, nil
}
func brief(episode store.Episode) Episode {
	return Episode{episode.ID, episode.Title, episode.PositionSeconds, episode.DurationSeconds, episode.Completed}
}

func readFrame(reader io.Reader, limit int, value any) error {
	line, err := bufio.NewReaderSize(reader, limit+1).ReadSlice('\n')
	if err != nil {
		return feedback.Wrap(feedback.Protocol, err)
	}
	if len(line) > limit {
		return feedback.New(feedback.Protocol, "invalid or oversized control frame")
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return feedback.Wrap(feedback.Protocol, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return feedback.New(feedback.Protocol, "extra control JSON")
	}
	return nil
}
func writeFrame(writer io.Writer, limit int, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data)+1 > limit {
		return fmt.Errorf("control response too large")
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}
