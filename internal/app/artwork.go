package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/activity"
	"podmutt/internal/artwork"
	"podmutt/internal/ui"
)

type imageFetcher interface {
	Image(context.Context, string, string) (image.Image, error)
}
type artLoaded struct {
	generation uint64
	img        image.Image
	err        error
}
type artAcknowledged struct{}
type artProbe struct{}
type artProbeTimeout struct{}
type artState struct {
	fetcher                             imageFetcher
	id                                  uint32
	kitty                               bool
	possible, probed                    bool
	graphicsOK, versionOK, probeExpired bool
	rendered, packet                    string
	lines                               []string
	key                                 string
	generation                          uint64
	cancel                              context.CancelFunc
	image                               image.Image
	complete                            bool
	sent                                string
	inFlight                            bool
}

func newArt() artState {
	return artState{fetcher: artwork.New(nil), id: uint32(rand.IntN(0xfffffe) + 1), possible: artwork.KittySupported(os.Getenv("TERM"), os.Getenv("KITTY_WINDOW_ID"), os.Getenv("TMUX"), os.Getenv("STY"))}
}
func (m libraryShell) artURLs() (string, string) {
	episode, ok := m.contextEpisode()
	if m.Model.Focus() == ui.Details && m.detailTarget != nil {
		episode, ok = *m.detailTarget, true
	}
	episodeURL, showURL := "", ""
	if ok {
		episodeURL = episode.ArtworkURL
	}
	if m.browse.active && m.browse.preview != nil && episode.ID <= 0 {
		showURL = m.browse.preview.Fetched.Document.ArtworkURL
	} else {
		id := m.selectedFeed()
		if ok {
			id = episode.FeedID
		}
		for _, source := range m.snapshot.Feeds {
			if source.ID == id {
				showURL = source.ArtworkURL
				break
			}
		}
	}
	return episodeURL, showURL
}
func (m libraryShell) probeArt() tea.Cmd {
	if !m.art.possible {
		return nil
	}
	ctx, id := m.s.ctx, m.art.id
	return tea.Batch(tea.RequestTerminalVersion, tea.Raw(fmt.Sprintf("\x1b_Ga=q,t=d,f=24,s=1,v=1,i=%d;AAAA\x1b\\", id)), func() tea.Msg {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return artProbeTimeout{}
		}
	})
}
func (m *libraryShell) cancelArt() {
	if m.art.cancel != nil {
		m.art.cancel()
		m.art.cancel = nil
		m.art.generation++
	}
}

// Missing/unavailable remote covers fall back locally, but cancellation never
// becomes a successful logo reply. Run both fetching and decoding off the UI loop.
func loadArtwork(ctx context.Context, fetcher imageFetcher, episode, show string) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var img image.Image
	var err error
	if episode != "" || show != "" {
		img, err = fetcher.Image(ctx, episode, show)
	}
	if canceled := ctx.Err(); canceled != nil {
		return nil, canceled
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	if img == nil || err != nil {
		img, err = artwork.Logo()
	}
	if canceled := ctx.Err(); canceled != nil {
		return nil, canceled
	}
	return img, err
}

func (m libraryShell) syncArt() (libraryShell, tea.Cmd) {
	if m.art.fetcher == nil {
		return m, nil
	}
	episode, show := m.artURLs()
	_, hasEpisode := m.contextEpisode()
	key := fmt.Sprintf("%t:%s\x00%s", hasEpisode, episode, show)
	if key != m.art.key {
		m.cancelArt()
		m.art.key = key
		m.art.image = nil
		m.art.complete = false
	}
	columns, rows := m.Model.ArtworkSize()
	visible := columns > 0 && rows > 0
	var load tea.Cmd
	if !visible {
		m.cancelArt()
	} else if !m.art.complete && m.art.cancel == nil {
		if episode == "" && show == "" && !hasEpisode {
			m.art.complete = true
		} else {
			ctx, cancel := context.WithCancel(m.s.ctx)
			m.art.cancel = cancel
			m.art.generation++
			version, fetcher := m.art.generation, m.art.fetcher
			load = trackedWork(ctx, "Loading artwork…", activity.Background, func(ctx context.Context) tea.Msg {
				img, err := loadArtwork(ctx, fetcher, episode, show)
				return artLoaded{version, img, err}
			})
		}
	}
	desired, packet := "", ""
	if !visible {
		m.Model = m.Model.WithArtwork(0, nil)
	} else {
		lines := strings.Split(artwork.ASCII(m.art.image, columns, rows), "\n")
		if m.art.kitty && m.art.image != nil {
			desired = fmt.Sprintf("%d:%d:%d", m.art.generation, columns, rows)
			if desired != m.art.rendered {
				m.art.packet, m.art.lines = artwork.Kitty(m.art.image, m.art.id, columns, rows)
				m.art.rendered = desired
			}
			packet, lines = m.art.packet, m.art.lines
		}
		m.Model = m.Model.WithArtwork(columns, lines)
	}
	var draw tea.Cmd
	if desired != m.art.sent && !m.art.inFlight {
		if m.art.sent != "" {
			packet = artwork.KittyDelete(m.art.id) + packet
		}
		m.art.sent = desired
		m.art.inFlight = true
		// One outstanding Raw operation. The acknowledgement is ordered AFTER Raw
		// delivery by Sequence; rapid selection/resize cannot reorder old uploads
		// behind newer deletions. Ack coalesces to the latest desired placement.
		draw = tea.Sequence(tea.Raw(packet), func() tea.Msg { return artAcknowledged{} })
	}
	return m, tea.Batch(load, draw)
}
