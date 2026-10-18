package player

import (
	"reflect"
	"testing"
)

func TestDecoderEnvironmentExcludesDirectoryCredentials(t *testing.T) {
	input := []string{"PATH=/bin", "PODMUTT_PODCAST_INDEX_KEY=private-key", "PODMUTT_PODCAST_INDEX_SECRET=private-secret", "TERM=xterm", "PODMUTT_MPV_HELPER=normal"}
	if got := mediaEnvironment(input); !reflect.DeepEqual(got, []string{"PATH=/bin", "TERM=xterm", "PODMUTT_MPV_HELPER=normal"}) {
		t.Fatal("decoder credential filtering failed")
	}
	if len(input) != 5 {
		t.Fatal("environment mutated")
	}
}
