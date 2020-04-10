package vterm

import (
	"github.com/aaronjanse/3mux/render"
)

func (v *VTerm) ScrollbackReset() {
	v.ScrollbackPos = 0

	v.RedrawWindow()
}

// ScrollbackUp shifts the screen contents up, with scrollback
func (v *VTerm) ScrollbackUp() {
	if v.usingAltScreen {
		return
	}

	if v.ScrollbackPos-5 >= 0 {
		v.ScrollbackPos -= 5
		v.implRedrawWindow(true)
	}
}

// ScrollbackDown shifts the screen contents down, with scrollback
func (v *VTerm) ScrollbackDown() {
	if v.usingAltScreen {
		return
	}

	if len(v.Scrollback) == 0 {
		return
	}

	if v.ScrollbackPos+5 < len(v.Scrollback) {
		v.ScrollbackPos += 5
		v.implRedrawWindow(true)
	}
}

// RefreshCursor refreshes the ncurses cursor position
func (v *VTerm) RefreshCursor() {
	if v.IsPaused {
		return
	}
	v.parentSetCursor(v.Cursor.X, v.Cursor.Y)
}

// scrollUp shifts screen contents up and adds blank lines to the bottom of the screen.
// Lines pushed out of view are put in the scrollback.
func (v *VTerm) scrollUp(n int) {
	if !v.usingAltScreen {
		rows := v.Screen[v.scrollingRegion.top : v.scrollingRegion.top+n]
		v.Scrollback = append(v.Scrollback, rows...)
	}

	newLines := make([][]render.Char, n)
	for i := range newLines {
		newLines[i] = make([]render.Char, v.w)
	}

	v.Screen = append(append(append(
		v.Screen[:v.scrollingRegion.top],
		v.Screen[v.scrollingRegion.top+n:v.scrollingRegion.bottom+1]...),
		newLines...),
		v.Screen[v.scrollingRegion.bottom+1:]...)

	if !v.usingSlowRefresh {
		v.RedrawWindow()
	}
}

// scrollDown shifts the screen content down and adds blank lines to the top.
// It does neither modifies nor reads scrollback
func (v *VTerm) scrollDown(n int) {
	newLines := make([][]render.Char, n)
	for i := range newLines {
		newLines[i] = make([]render.Char, v.w)
	}

	v.Screen =
		append(v.Screen[:v.scrollingRegion.top],
			append(newLines,
				append(v.Screen[v.scrollingRegion.top:v.scrollingRegion.bottom+1-n],
					v.Screen[v.scrollingRegion.bottom+1:]...)...)...)

	if !v.usingSlowRefresh {
		v.RedrawWindow()
	}
}

func (v *VTerm) setCursorPos(x, y int) {
	// TODO: account for scrolling positon

	if x < 0 {
		v.Cursor.X = 0
	} else if x > v.w {
		v.Cursor.X = v.w
	} else {
		v.Cursor.X = x
	}

	if y < 0 {
		v.Cursor.Y = 0
	} else if y > v.h {
		v.Cursor.Y = v.h
	} else {
		v.Cursor.Y = y
	}

	v.RefreshCursor()
}

func (v *VTerm) setCursorX(x int) {
	v.setCursorPos(x, v.Cursor.Y)
}

func (v *VTerm) setCursorY(y int) {
	v.setCursorPos(v.Cursor.X, y)
}

func (v *VTerm) shiftCursorX(diff int) {
	v.setCursorPos(v.Cursor.X+diff, v.Cursor.Y)
}

func (v *VTerm) shiftCursorY(diff int) {
	v.setCursorPos(v.Cursor.X, v.Cursor.Y+diff)
}

// putChar renders as given character using the cursor stored in vterm
func (v *VTerm) putChar(ch rune) {
	if v.Cursor.Y >= v.h || v.Cursor.Y < 0 || v.Cursor.X > v.w || v.Cursor.X < 0 {
		return
	}

	char := render.Char{
		Rune:  ch,
		Style: v.Cursor.Style,
	}

	if v.Cursor.Y >= 0 && v.Cursor.Y < len(v.Screen) {
		if v.Cursor.X >= 0 && v.Cursor.X < len(v.Screen[v.Cursor.Y]) {
			v.Screen[v.Cursor.Y][v.Cursor.X] = char
		}
	}

	positionedChar := render.PositionedChar{
		Rune:   ch,
		Cursor: v.Cursor,
	}

	positionedChar.Cursor.X += v.x
	positionedChar.Cursor.Y += v.y

	// TODO: print to the window based on scrolling position
	v.renderer.HandleCh(positionedChar)

	if v.Cursor.X < v.w {
		v.Cursor.X++
	}

	v.RefreshCursor()
}

// RedrawWindow redraws the screen into ncurses from scratch.
// This should be reserved for operations not yet formalized into a generic, efficient function.
func (v *VTerm) RedrawWindow() {
	v.implRedrawWindow(false)
}

func (v *VTerm) implRedrawWindow(force bool) {
	if v.ScrollbackPos < v.h {
		for y := 0; y < v.h-v.ScrollbackPos; y++ {
			for x := 0; x < v.w; x++ {
				if y >= len(v.Screen) || x >= len(v.Screen[y]) {
					continue
				}

				ch := render.PositionedChar{
					Rune: v.Screen[y][x].Rune,
					Cursor: render.Cursor{
						X: v.x + x, Y: v.y + y + v.ScrollbackPos, Style: v.Screen[y][x].Style,
					},
				}

				if force {
					v.renderer.ForceHandleCh(ch)
				} else {
					v.renderer.HandleCh(ch)
				}
			}
		}
	}

	if !v.usingSlowRefresh {
		v.RefreshCursor()
	}

	if v.ScrollbackPos > 0 {
		numLinesVisible := v.ScrollbackPos
		if v.ScrollbackPos > v.h {
			numLinesVisible = v.h
		}
		for y := 0; y < numLinesVisible; y++ {
			for x := 0; x < v.w; x++ {
				idx := len(v.Scrollback) - v.ScrollbackPos + y - 1

				if x < len(v.Scrollback[idx]) {
					ch := render.PositionedChar{
						Rune: v.Scrollback[idx][x].Rune,
						Cursor: render.Cursor{
							X: v.x + x, Y: v.y + y, Style: v.Scrollback[idx][x].Style,
						},
					}
					if force {
						v.renderer.ForceHandleCh(ch)
					} else {
						v.renderer.HandleCh(ch)
					}
				} else {
					ch := render.PositionedChar{
						Rune: ' ',
						Cursor: render.Cursor{
							X: v.x + x, Y: v.y + y, Style: render.Style{},
						},
					}
					if force {
						v.renderer.ForceHandleCh(ch)
					} else {
						v.renderer.HandleCh(ch)
					}
				}
			}
		}
	}
}
