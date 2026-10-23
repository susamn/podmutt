package player

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"podmutt/internal/chapters"
)

func TestRealMPVEmbeddedChaptersAndPausedSeek(t *testing.T) {
	binary := integrationBinary(t)
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg required to generate chaptered audio fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	wav := silenceWAV(t, 4)
	dir := t.TempDir()
	metadata := filepath.Join(dir, "chapters.ffmeta")
	if err := os.WriteFile(metadata, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1500\ntitle=Opening\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=1500\nEND=4000\ntitle=Main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(dir, "chaptered.mka")
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-loglevel", "error", "-i", wav, "-i", metadata, "-map_metadata", "1", "-c:a", "flac", "-y", media)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture generation: %v: %s", err, output)
	}
	args := append(append([]string{}, arguments...), "--ao=null")
	p, err := launch(ctx, exec.CommandContext(ctx, binary, args...), shutdownTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Load(ctx, media, 0); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, p, "file-loaded", "")
	if err := p.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	raw, err := p.ChapterList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	list, err := chapters.FromMPV(raw)
	if err != nil || len(list) != 2 || list[0].Title != "Opening" || list[1].Title != "Main" || list[1].Start < 1.4 || list[1].Start > 1.6 {
		t.Fatalf("embedded chapter list: %s => %+v, %v", raw, list, err)
	}
	if err := p.SeekTo(ctx, list[1].Start); err != nil {
		t.Fatal(err)
	}
	paused, err := p.command(ctx, []any{"get_property", "pause"})
	if err != nil || string(paused) != "true" {
		t.Fatal("chapter jump resumed paused audio", string(paused), err)
	}
}
