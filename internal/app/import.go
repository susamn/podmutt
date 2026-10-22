package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"podmutt/internal/feed"
	"podmutt/internal/feedback"
	"podmutt/internal/library"
	"podmutt/internal/opml"
	"podmutt/internal/profile"
	"podmutt/internal/store"
)

const importWorkers = 4

// ImportOPML subscribes to every feed in an OPML file. It owns the profile like
// the TUI does, so it cannot run while podmutt is open.
func ImportOPML(path string, output io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return feedback.Wrap(feedback.Import, err)
	}
	feeds, err := opml.Parse(file)
	file.Close()
	if err != nil {
		return feedback.Wrap(feedback.Import, err)
	}
	if len(feeds) == 0 {
		return feedback.New(feedback.Import, "no podcast feeds found in this OPML file")
	}
	return withProfile(func(cfg profile.Config, paths profile.Paths) (err error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		db, err := store.Open(ctx, filepath.Join(paths.State, "podcasts.db"))
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		service := library.New(db, feed.New(db, nil, time.Duration(cfg.RefreshMinutes)*time.Minute))
		failed := importFeeds(ctx, feeds, func(ctx context.Context, url string) error {
			_, err := service.Add(ctx, url)
			return err
		}, output, importWorkers)
		if failed < len(feeds) {
			if err := db.SeedAutoqueueSeen(ctx, 0); err != nil {
				return &feedback.Fault{Code: feedback.Import, Safe: "Subscriptions were saved, but import bookkeeping could not be saved. Check free disk space and profile permissions before retrying.", Cause: err}
			}
		}
		if failed > 0 {
			return feedback.New(feedback.Import, fmt.Sprintf("%d of %d feeds could not be imported", failed, len(feeds)))
		}
		return nil
	})
}

// importFeeds adds feeds concurrently, prints one line per feed in input order
// and returns the number of failures.
func importFeeds(ctx context.Context, feeds []opml.Feed, add func(context.Context, string) error, output io.Writer, workers int) int {
	errs := make([]error, len(feeds))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range max(1, workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				errs[i] = add(ctx, feeds[i].URL)
			}
		}()
	}
	for i := range feeds {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	failed := 0
	for i, item := range feeds {
		name := feedback.PublicLabel(item.Title)
		if name == "" {
			name = fmt.Sprintf("Feed %d", i+1)
		}
		if errs[i] != nil {
			failed++
			fmt.Fprintf(output, "FAIL  %s: %s\n", name, feedback.For("Import feed", errs[i]).Text())
			continue
		}
		fmt.Fprintf(output, "ok    %s\n", name)
	}
	fmt.Fprintf(output, "%d feeds processed successfully, %d failed\n", len(feeds)-failed, failed)
	return failed
}
