// Pixel-pet sprite renderer for `llm-recall stats`.
//
// W9 ships a 16×16 character pet that picks one of 7 expressions based
// on the user's recent activity. The pet renders in the empty space to
// the right of the heatmap when the terminal is wide enough; below
// 100 columns we fall back to the W5 vertical layout and skip the pet
// entirely (squeezing it on top of heatmap cells would be visual noise
// for the small benefit of "more cute" — easy call).
//
// Sprite design constraints:
//   - Each sprite is exactly 16 lines tall, ≤ 16 cells wide.
//   - All cells are 1-column wide ASCII / Latin / box-drawing chars
//     so runewidth math doesn't have to special-case them. The eye /
//     decoration glyphs (◉ ⌒ ✦ ★ ?) are 1-cell in every modern
//     terminal we test against.
//   - Background is the body fill — we render the silhouette over an
//     opaque colour rectangle in lipgloss so the sprite has a "card"
//     look without per-cell background painting.
//   - White ghost body fill, dark eyes; the lipgloss layer wires up
//     the colour palette specified in TASKS-W9.md §3.2 (#F5F5F5 fluff,
//     #1a1a1a eye, #1a3a6e backdrop).
//
// Why slices of strings rather than [][]rune: the strings are easy to
// proofread visually in this file, easy to grep, and lipgloss's
// JoinHorizontal takes []string anyway.
package stats

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// State enumerates the 7 sprite expressions. ChooseState() picks one
// based on Stats fields.
type State int

const (
	Idle State = iota
	Happy
	Pumped
	Sad
	Sleeping
	Confused
	Cheering
)

// PetWidth / PetHeight are the rendered cell extents. Public so the
// stats render layout can reserve space without allocating the sprite.
const (
	PetWidth  = 16
	PetHeight = 16
)

// Sprites maps each State to its 16-line ASCII art. Each sprite is
// padded to PetWidth columns so JoinHorizontal lines up with the
// heatmap rows on the left.
//
// Reading guide:
//   - `█` body fill / fluff
//   - `▀ ▄ ▌ ▐` rounded corners
//   - ` ` (space) backdrop / no fluff
//   - eye / decoration glyphs follow the spec column in the table:
//     Idle → single ● looking left
//     Happy → ⌒ ⌒ smiling eye-arcs
//     Pumped → ✦ ✦ sparkly eyes
//     Sad → ◠ ◠ drooping (we use ︵ ︵ for emphasis below the brow)
//     Sleeping → -- closed eyes + zZz floating above
//     Confused → ?? above eye area
//     Cheering → ★ ★ stars + paws raised (extra bumps in the silhouette)
var Sprites = map[State][]string{
	Idle: {
		"                ",
		"     ▄████▄     ",
		"   ▄████████▄   ",
		"  ██████████▄   ",
		" ████████████   ",
		" ████████████   ",
		" ███●████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" █▀█▀█▀█▀█▀█▀   ",
		"                ",
		"                ",
	},
	Happy: {
		"                ",
		"     ▄████▄     ",
		"   ▄████████▄   ",
		"  ██████████▄   ",
		" ████████████   ",
		" ████████████   ",
		" ██⌒██████⌒██   ",
		" ████████████   ",
		" ████████████   ",
		" ██████◡█████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" █▀█▀█▀█▀█▀█▀   ",
		"                ",
		"                ",
	},
	Pumped: {
		"     ✦   ✦      ",
		"      ▄██▄      ",
		"    ▄██████▄    ",
		"   ██████████   ",
		"  ████████████  ",
		"  ████████████  ",
		"  ██✦██████✦██  ",
		"  ████████████  ",
		"  ██████◡█████  ",
		"  ████████████  ",
		"  ████████████  ",
		"  ████████████  ",
		"  ████████████  ",
		"  █▀█▀█▀█▀█▀█▀  ",
		"     ✦   ✦      ",
		"                ",
	},
	Sad: {
		"                ",
		"     ▄████▄     ",
		"   ▄████████▄   ",
		"  ██████████▄   ",
		" ████████████   ",
		" █╲██████╱██    ",
		" ██●██████●██   ",
		" ████████████   ",
		" ████████████   ",
		" ██████︵█████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" █▀█▀█▀█▀█▀█▀   ",
		"        ,       ",
		"                ",
	},
	Sleeping: {
		"          z     ",
		"        z Z     ",
		"     ▄████▄     ",
		"   ▄████████▄   ",
		"  ██████████▄   ",
		" ████████████   ",
		" ██──██████──█  ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" █▀█▀█▀█▀█▀█▀   ",
		"                ",
		"                ",
	},
	Confused: {
		"        ?       ",
		"     ▄████▄  ?  ",
		"   ▄████████▄   ",
		"  ██████████▄   ",
		" ████████████   ",
		" ████████████   ",
		" ██?██████?██   ",
		" ████████████   ",
		" ████████████   ",
		" ██████○█████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" █▀█▀█▀█▀█▀█▀   ",
		"                ",
		"                ",
	},
	Cheering: {
		"   ★        ★   ",
		"     ▄████▄     ",
		" ▌  ▄████████▄ ▐",
		" ▌ ██████████▄ ▐",
		" ▌████████████▐ ",
		" ████████████   ",
		" ██●██████●██   ",
		" ████████████   ",
		" ██████◡█████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" ████████████   ",
		" █▀█▀█▀█▀█▀█▀   ",
		"      ★ ★       ",
		"                ",
	},
}

// ChooseState picks a sprite for the supplied Stats. Order matters —
// Sleeping wins over everything else (so a 30-day idle user sees the
// sleeping ghost instead of "sad"), Confused next (data anomaly is a
// tooling issue we want surfaced), then the active states.
func ChooseState(s Stats) State {
	switch {
	case s.RecentDays7 == 0 && s.Sessions > 0:
		// Has historical sessions but nothing in 7 days → asleep.
		return Sleeping
	case s.AnomalousData:
		return Confused
	case s.LongestStreak >= 14 || s.SessionsToday >= 10 || s.CurrentStreak >= 14:
		return Pumped
	case s.CurrentStreak >= 7 || s.LongestStreak >= 7:
		return Happy
	case s.HasRecordToday:
		return Cheering
	case s.ActiveDays < 5:
		return Sad
	default:
		return Idle
	}
}

// RenderPet returns the styled multi-line string for `state`. Lines
// are right-padded to PetWidth so JoinHorizontal in the parent
// renderer aligns to a known column width.
func RenderPet(state State) string {
	lines, ok := Sprites[state]
	if !ok {
		lines = Sprites[Idle]
	}
	body := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F5F5F5")).
		Background(lipgloss.Color("#1a3a6e")).
		Padding(0, 1)
	var b strings.Builder
	for _, l := range lines {
		// Pad to PetWidth so the lipgloss background fills uniformly.
		padded := padRightVisible(l, PetWidth)
		b.WriteString(padded)
		b.WriteString("\n")
	}
	return body.Render(strings.TrimRight(b.String(), "\n"))
}

// padRightVisible right-pads `s` so its visible cell count equals n.
// We can't use len() because some glyphs (e.g. ✦) are 1-cell but
// multi-byte. Counting by rune is "good enough" for the ≤16-rune
// strings used in the sprite tables.
func padRightVisible(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}
