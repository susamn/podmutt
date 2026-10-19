package ui

import (
	"math"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"podmutt/internal/theme"
)

type visualizerEngine struct {
	mu sync.Mutex
}

var globalVisualizer visualizerEngine

func hairlineBandLevel(t float64, x float64, maxLevel float64) float64 {
	v1 := math.Sin(t*3.0+x*4.0)*0.5 + 0.5
	v2 := math.Cos(t*2.0-x*7.0)*0.35 + 0.35
	v3 := math.Sin(t*5.5+x*12.0)*0.25 + 0.25
	combined := v1*0.45 + v2*0.35 + v3*0.2
	return combined * maxLevel
}

func (v *visualizerEngine) Render(width, height int, t float64, status string, pal theme.Palette) []string {
	if height <= 0 || width <= 0 {
		return make([]string, height)
	}
	height = min(2, height)
	lines := make([]string, height)

	v.mu.Lock()
	defer v.mu.Unlock()

	barWidth := 1
	gap := 2
	if width < 50 {
		gap = 1
	}

	numBars := (width + gap) / (barWidth + gap)
	if numBars < 1 {
		numBars = 1
	}

	totalUsed := numBars*barWidth + (numBars-1)*gap
	padLeft := max(0, (width-totalUsed)/2)
	padRight := max(0, width-totalUsed-padLeft)

	padLeftStr := strings.Repeat(" ", padLeft)
	padRightStr := strings.Repeat(" ", padRight)
	gapStr := strings.Repeat(" ", gap)

	maxLevel := float64(height * 2)
	levels := make([]int, numBars)

	if status == "playing" || status == "paused" {
		for b := 0; b < numBars; b++ {
			posRatio := float64(b) / float64(max(1, numBars-1))
			lvl := hairlineBandLevel(t, posRatio, maxLevel)
			levels[b] = int(math.Round(lvl))
			if levels[b] > int(maxLevel) {
				levels[b] = int(maxLevel)
			}
			if levels[b] < 0 {
				levels[b] = 0
			}
		}
	}

	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Cyan)))
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Accent)))

	if height == 1 {
		var sb strings.Builder
		sb.WriteString(padLeftStr)
		for b := 0; b < numBars; b++ {
			if b > 0 {
				sb.WriteString(gapStr)
			}
			char := " "
			switch levels[b] {
			case 1:
				char = "╵"
			case 2:
				char = "│"
			}
			if char != " " {
				sb.WriteString(cyanStyle.Render(char))
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString(padRightStr)
		lines[0] = sb.String()
		return lines
	}

	for y := 0; y < 2; y++ {
		var sb strings.Builder
		sb.WriteString(padLeftStr)
		for b := 0; b < numBars; b++ {
			if b > 0 {
				sb.WriteString(gapStr)
			}
			lvl := levels[b]
			char := " "
			if y == 0 {
				switch lvl {
				case 3:
					char = "╷"
				case 4:
					char = "│"
				}
			} else {
				switch {
				case lvl >= 2:
					char = "│"
				case lvl == 1:
					char = "╵"
				}
			}
			if char != " " {
				if y == 0 {
					sb.WriteString(accentStyle.Render(char))
				} else {
					sb.WriteString(cyanStyle.Render(char))
				}
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString(padRightStr)
		lines[y] = sb.String()
	}

	return lines
}
