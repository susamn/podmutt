package app

import (
	"context"
	"errors"
	"path/filepath"

	"podmutt/internal/control"
	"podmutt/internal/macmedia"
	"podmutt/internal/mpris"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/profile"
	"podmutt/internal/store"
	"podmutt/internal/theme"
	"podmutt/internal/ui"
)

type session struct {
	ctx         context.Context
	config      profile.Config
	paths       profile.Paths
	store       *store.Store
	playback    *playback.Engine
	nativeMedia *macmedia.Service
	media       *mpris.Service
	mediaError  error
	theme       theme.Palette
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
	native, nativeError := macmedia.Start(ctx, engine)
	if native != nil {
		defer native.Close()
	}
	pal, _ := theme.Load(paths.ConfigDir(), cfg.Theme)
	return run(&session{ctx: ctx, config: cfg, paths: paths, store: db, playback: engine, media: media, nativeMedia: native, mediaError: errors.Join(mediaError, nativeError), theme: pal})
}

func (s *session) settingsData() ui.SettingsData {
	_, activeThemePath := theme.Load(s.paths.ConfigDir(), s.config.Theme)
	return ui.SettingsData{
		ActiveTheme: s.config.Theme,
		Themes:      theme.Available(s.paths.ConfigDir()),
		ConfigPath:  s.paths.Config,
		ThemePath:   activeThemePath,
		MPVPath:     s.config.MPVPath,
		RefreshMin:  s.config.RefreshMinutes,
		Icons:       s.config.Icons,
	}
}
