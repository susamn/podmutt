package ui

import (
	"math"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"podmutt/internal/theme"
)

var equalizerBlocks = []rune{' ', ' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type visualizerEngine struct {
	mu        sync.Mutex
	peaks     []float64
	peakTimes []float64
	lastTime  float64
}

var globalVisualizer visualizerEngine

func cliampBandLevel(t float64, b, numBars int, maxLevel float64) float64 {
	x := float64(b) / float64(max(1, numBars-1))
	bass := math.Sin(t*7.5) * math.Cos(t*3.2)
	bass = math.Max(0, bass*bass*bass) * (1.0 - x*0.8)
	mids := math.Sin(t*4.0+x*6.0)*0.5 + math.Cos(t*2.5-x*4.0)*0.5
	mids = math.Max(0, mids) * math.Sin(x*math.Pi)
	treble := math.Sin(t*18.0+x*12.0)*0.5 + math.Cos(t*24.0-x*8.0)*0.5
	treble = math.Max(0, treble*treble) * x

	combined := bass*0.6 + mids*0.4 + treble*0.35
	level := combined * maxLevel
	if level < 0 {
		level = 0
	}
	if level > maxLevel {
		level = maxLevel
	}
	return level
}

func (v *visualizerEngine) Render(width, height int, t float64, status string, pal theme.Palette) []string {
	lines := make([]string, height)
	if width <= 0 || height <= 0 {
		return lines
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if t < v.lastTime {
		v.peaks = nil
		v.peakTimes = nil
	}
	v.lastTime = t

	barWidth := 2
	gap := 1
	if width < 24 {
		barWidth = 1
		gap = 0
	} else if width < 40 {
		barWidth = 1
		gap = 1
	}

	numBars := (width + gap) / (barWidth + gap)
	if numBars < 1 {
		numBars = 1
	}

	if len(v.peaks) != numBars {
		v.peaks = make([]float64, numBars)
		v.peakTimes = make([]float64, numBars)
	}

	maxLevel := height * 8
	maxLevelF := float64(maxLevel)

	isPlaying := status == "playing"
	isPaused := status == "paused"

	barLevels := make([]int, numBars)
	peakLevels := make([]int, numBars)

	for b := 0; b < numBars; b++ {
		var target float64
		if isPlaying {
			target = cliampBandLevel(t, b, numBars, maxLevelF)
		} else if isPaused {
			target = v.peaks[b] * 0.7
		} else {
			target = 0
		}

		if target >= v.peaks[b] {
			v.peaks[b] = target
			v.peakTimes[b] = t
		} else if isPlaying {
			dt := t - v.peakTimes[b]
			if dt > 0.12 {
				decay := 14.0 * (dt - 0.12)
				v.peaks[b] -= decay
				if v.peaks[b] < target {
					v.peaks[b] = target
				}
				v.peakTimes[b] = t - 0.12
			}
		} else if !isPaused {
			v.peaks[b] = 0
		}

		barLevels[b] = int(math.Round(target))
		if barLevels[b] > maxLevel {
			barLevels[b] = maxLevel
		}
		if barLevels[b] < 0 {
			barLevels[b] = 0
		}

		peakLevels[b] = int(math.Round(v.peaks[b]))
		if peakLevels[b] > maxLevel {
			peakLevels[b] = maxLevel
		}
		if peakLevels[b] < 0 {
			peakLevels[b] = 0
		}
	}

	totalUsed := numBars*barWidth + (numBars-1)*gap
	padLeft := max(0, (width-totalUsed)/2)
	padRight := max(0, width-totalUsed-padLeft)

	padLeftStr := strings.Repeat(" ", padLeft)
	padRightStr := strings.Repeat(" ", padRight)
	gapStr := strings.Repeat(" ", gap)

	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Green)))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Yellow)))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Red)))
	peakStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.BrightRed)))

	for y := 0; y < height; y++ {
		var sb strings.Builder
		sb.WriteString(padLeftStr)

		rowFromBottom := height - 1 - y
		rowBaseLevel := rowFromBottom * 8

		for b := 0; b < numBars; b++ {
			if b > 0 && gap > 0 {
				sb.WriteString(gapStr)
			}

			barL := barLevels[b]
			peakL := peakLevels[b]

			rowFilled := barL - rowBaseLevel
			if rowFilled > 8 {
				rowFilled = 8
			} else if rowFilled < 0 {
				rowFilled = 0
			}

			if rowFilled > 0 {
				glyph := equalizerBlocks[rowFilled]
				barStr := strings.Repeat(string(glyph), barWidth)
				if barL > (maxLevel*2)/3 {
					sb.WriteString(redStyle.Render(barStr))
				} else if barL > maxLevel/3 {
					sb.WriteString(yellowStyle.Render(barStr))
				} else {
					sb.WriteString(greenStyle.Render(barStr))
				}
			} else {
				peakInRow := peakL > rowBaseLevel && peakL <= rowBaseLevel+8
				if peakInRow && peakL > barL {
					sb.WriteString(peakStyle.Render(strings.Repeat("▔", barWidth)))
				} else {
					sb.WriteString(strings.Repeat(" ", barWidth))
				}
			}
		}

		sb.WriteString(padRightStr)
		lines[y] = sb.String()
	}

	return lines
}
