package render

import "fmt"

// Cursor is Style along with position
type Cursor struct {
	X, Y int
	Style
}

// Style is the state of the terminal's drawing modes when printing a given character
type Style struct {
	Bold, Faint, Italic, Underline, Conceal, CrossedOut bool

	Fg Color // foreground color
	Bg Color // background color
}

// Reset sets all rendering attributes of a cursor to their default values
func (s *Style) Reset() {
	s.Bold = false
	s.Faint = false
	s.Italic = false
	s.Underline = false
	s.Conceal = false
	s.CrossedOut = false

	s.Fg.ColorMode = ColorNone
	s.Bg.ColorMode = ColorNone
}

// deltaMarkup returns markup to transform from one cursor to another
func deltaMarkup(fromCur, toCur Cursor) string {
	out := ""

	/* update position */

	xDiff := toCur.X - fromCur.X
	yDiff := toCur.Y - fromCur.Y

	if xDiff == 0 && yDiff == 1 {
		out += "\n"
	} else if xDiff != 0 || yDiff != 0 {
		out += fmt.Sprintf("\033[%d;%dH", toCur.Y+1, toCur.X+1)
	}

	/* update colors */

	to := toCur.Style
	from := fromCur.Style

	if to.Bg.ColorMode != from.Bg.ColorMode || to.Bg.Code != from.Bg.Code {
		out += to.Bg.ToANSI(true)
	}

	if to.Fg.ColorMode != from.Fg.ColorMode || to.Fg.Code != from.Fg.Code {
		out += to.Fg.ToANSI(false)
	}

	// Without this, text randomly gets randomly underlined for some reason.
	if !to.Underline && !from.Underline {
		out += "\033[24m"
	}

	/* remove effects */

	if from.Faint && !to.Faint {
		out += "\033[22m"
	}

	if from.Underline && !to.Underline {
		out += "\033[24m"
	}

	/* add effects */

	if !from.Faint && to.Faint {
		out += "\033[2m"
	}

	if !from.Underline && to.Underline {
		out += "\033[4m"
	}

	return out
}
