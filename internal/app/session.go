package app

import (
	"context"
	"errors"
	"path/filepath"

	"podmutt/internal/control"
	"podmutt/internal/mpris"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/profile"
	"podmutt/internal/store"
)

type session struct {
	ctx        context.Context
	config     profile.Config
	paths      profile.Paths
	store      *store.Store
	playback   *playback.Engine
	media      *mpris.Service
	mediaError error
}

// withSession runs under withProfile ownership. All foreground services share
// this database, actor, and cancellation lifetime; no client creates its own.
func withSession(cfg profile.Config, paths profile.Paths, run func(*session) error) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, err := store.Open(ctx, filepath.Join(paths.State, "podcasts.db"))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	engine, err := playback.New(ctx, db, func(ctx context.Context) (playback.Player, error) { return player.Start(ctx, cfg.MPVPath) })
	if err != nil {
		return err
	}
	defer func() { cancel(); err = errors.Join(err, engine.Close(context.Background())) }()
	server, err := control.Listen(ctx, paths, engine)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, server.Close()) }()
	media, mediaError := mpris.Start(ctx, engine)
	if media != nil {
		defer media.Close()
	}
	return run(&session{ctx: ctx, config: cfg, paths: paths, store: db, playback: engine, media: media, mediaError: mediaError})
}
