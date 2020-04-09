package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/aaronjanse/i3-tmux/render"
	"github.com/aaronjanse/i3-tmux/vterm"
)

// SearchDirection is which direction we move through search results
type SearchDirection int

// enum of search directions
const (
	SearchUp SearchDirection = iota
	SearchDown
)

// A Pane is a tiling unit representing a terminal
type Pane struct {
	id int

	selected bool

	renderRect Rect

	vterm *vterm.VTerm
	shell Shell

	searchMode            bool
	searchText            string
	searchPos             int
	searchBackupScrollPos int
	searchDidShiftUp      bool
	searchResultsMode     bool
	searchDirection       SearchDirection

	Dead bool
}

func newTerm(selected bool) *Pane {
	stdout := make(chan rune, 3200000)
	stdin := make(chan rune, 3200000)

	t := &Pane{
		id:       rand.Intn(10),
		selected: selected,

		shell: newShell(stdout),
	}

	go func() {
		for {
			x := <-stdin
			t.shell.handleStdin(string(x))
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fatalShutdownNow("pane.go (shell death)\n" + r.(error).Error())
			}
		}()

		t.shell.cmd.Wait()
		t.Dead = true
		removeTheDead([]int{root.selectionIdx})

		if len(root.workspaces[root.selectionIdx].contents.elements) == 0 {
			shutdownNow()
		} else {
			// deselect the old Term
			newTerm := getSelection().getContainer().(*Pane)
			newTerm.selected = true
			newTerm.softRefresh()
			newTerm.vterm.RefreshCursor()

			root.simplify()
			root.refreshRenderRect()
		}
	}()

	parentSetCursor := func(x, y int) {
		if t.selected {
			renderer.SetCursor(x+t.renderRect.x, y+t.renderRect.y)
		}
	}

	vt := vterm.NewVTerm(&t.shell.byteCounter, renderer, parentSetCursor, stdout, stdin)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fatalShutdownNow("pane.go (vt.ProcessStream)\n" + r.(error).Error())
			}
		}()
		vt.ProcessStream()
	}()

	t.vterm = vt

	return t
}

func (t *Pane) handleStdin(in string) {
	if t.searchMode && t.searchResultsMode {
		switch in[0] { // FIXME ignores extra chars
		case 'n': // next
			t.searchDirection = SearchDown
			t.searchPos--
			if t.searchPos < 0 {
				t.searchPos = 0
			}
			t.doSearch()
		case 'N': // prev
			t.searchDirection = SearchUp
			t.searchPos++
			max := len(t.vterm.Scrollback) + len(t.vterm.Screen) - 1
			if t.searchPos > max {
				t.searchPos = max
			}
			t.doSearch()
		case '/':
			t.searchResultsMode = false
			t.displayStatusText(t.searchText)
		case 127:
			fallthrough
		case 8:
			t.searchResultsMode = false
			t.searchText = t.searchText[:len(t.searchText)-1]
			t.displayStatusText(t.searchText)
		case 13:
			fallthrough
		case 10: // enter
			t.toggleSearch()
			t.vterm.ScrollbackPos = t.searchPos - len(t.vterm.Screen) + t.renderRect.h/2
			t.vterm.RedrawWindow()
		}
	} else if t.searchMode {
		for _, c := range in {
			if c == 8 || c == 127 { // backspace
				if len(t.searchText) > 0 {
					t.searchText = t.searchText[:len(t.searchText)-1]
				}
			} else if c == 10 || c == 13 {
				if len(t.searchText) == 0 {
					t.toggleSearch()
					return
				} else {
					t.searchResultsMode = true
					return // FIXME ignores extra chars
				}
			} else {
				t.searchText += string(c)
			}
		}
		t.searchPos = 0
		t.doSearch()
		t.displayStatusText(t.searchText)
	} else {
		t.vterm.ScrollbackReset()
		t.shell.handleStdin(in)
		t.vterm.RefreshCursor()
	}
}

func (t *Pane) doSearch() {
	fullBuffer := append(t.vterm.Scrollback, t.vterm.Screen...)
	match, err := t.locateText(fullBuffer, t.searchText)

	if err == nil {
		t.searchPos = match.y1

		bottomOfScreen := 0
		if match.y1 > t.renderRect.h {
			topOfScreen := match.y1 + t.renderRect.h/2
			if topOfScreen > len(fullBuffer) { // top of scrollback
				topOfScreen = len(fullBuffer) - 1
				t.vterm.ScrollbackPos = len(t.vterm.Scrollback) - 1
			} else {
				t.vterm.ScrollbackPos = topOfScreen - t.renderRect.h - 1
			}
			bottomOfScreen = topOfScreen - t.renderRect.h
			match.y1 -= bottomOfScreen
			match.y2 -= bottomOfScreen
		} else {
			t.vterm.ScrollbackPos = 0
		}

		t.vterm.RedrawWindow()

		for i := match.x1; i <= match.x2; i++ {
			theY := len(fullBuffer) - (bottomOfScreen + match.y1 + 1)
			renderer.ForceHandleCh(render.PositionedChar{
				Rune: fullBuffer[theY][i].Rune,
				Cursor: render.Cursor{
					X: t.renderRect.x + i,
					Y: t.renderRect.y + t.renderRect.h - match.y1,
					Style: render.Style{
						Bg: render.Color{
							ColorMode: render.ColorBit3Bright,
							Code:      2,
						},
						Fg: render.Color{
							ColorMode: render.ColorBit3Normal,
							Code:      0,
						},
					},
				},
			})
		}
	} else {
		log.Println("Could not find match!")
	}
}

// SearchMatch coordinates are relative to bottom left. 1st coords are upper left and 2nd coords are bottom right of search match
type SearchMatch struct {
	x1, y1, x2, y2 int
}

func (t *Pane) locateText(chars [][]render.Char, text string) (SearchMatch, error) {
	lineFromBottom := t.searchPos

	i := len(chars) - t.searchPos - 1
	for {
		var str strings.Builder

		for _, c := range chars[i] {
			str.WriteRune(c.Rune)
		}

		pos := strings.Index(str.String(), text)
		if pos != -1 {
			return SearchMatch{
				x1: pos,
				x2: pos + len(text) - 1,
				y1: lineFromBottom,
				y2: lineFromBottom,
			}, nil
		}
		if t.searchDirection == SearchUp {
			lineFromBottom++
			i--
			if i < 0 {
				break
			}
		} else {
			lineFromBottom--
			i++
			if i >= len(chars) {
				break
			}
		}
	}

	return SearchMatch{}, errors.New("could not find match")
}

func (t *Pane) toggleSearch() {
	t.searchMode = !t.searchMode

	if t.searchMode {
		t.vterm.ChangePause <- true
		t.searchBackupScrollPos = t.vterm.ScrollbackPos
		t.searchResultsMode = false
		t.searchDirection = SearchUp

		// FIXME hacky way to wait for full control of screen section
		timer := time.NewTimer(time.Millisecond * 5)
		select {
		case <-timer.C:
			timer.Stop()
		}

		lastLineIsBlank := true
		lastLine := t.vterm.Screen[len(t.vterm.Screen)-2]
		for _, c := range lastLine {
			if c.Rune != 32 && c.Rune != 0 {
				lastLineIsBlank = false
				break
			}
		}

		t.searchDidShiftUp = !lastLineIsBlank

		if !lastLineIsBlank {
			blankLine := []render.Char{}
			for i := 0; i < t.renderRect.w; i++ {
				blankLine = append(blankLine, render.Char{Rune: ' ', Style: render.Style{}})
			}

			t.vterm.Scrollback = append(t.vterm.Scrollback, t.vterm.Screen[0])
			t.vterm.Screen = append(t.vterm.Screen[1:], blankLine)

			t.vterm.RedrawWindow()
		}

		t.displayStatusText("Search...")
	} else {
		t.clearStatusText()

		t.vterm.ScrollbackPos = t.searchBackupScrollPos

		if t.searchDidShiftUp {
			t.vterm.Screen = append([][]render.Char{t.vterm.Scrollback[len(t.vterm.Scrollback)-1]}, t.vterm.Screen[:len(t.vterm.Screen)-1]...)
			t.vterm.Scrollback = t.vterm.Scrollback[:len(t.vterm.Scrollback)-1]
		}
		t.vterm.RedrawWindow()
		t.vterm.ChangePause <- false
	}
}

func (t *Pane) displayStatusText(s string) {
	for i := 0; i < t.renderRect.w; i++ {
		r := ' '
		if i < len(s) {
			r = rune(s[i])
		}

		ch := render.PositionedChar{
			Rune: r,
			Cursor: render.Cursor{
				X: t.renderRect.x + i,
				Y: t.renderRect.y + t.renderRect.h - 1,
				Style: render.Style{
					Bg: render.Color{
						ColorMode: render.ColorBit3Bright,
						Code:      2,
					},
					Fg: render.Color{
						ColorMode: render.ColorBit3Normal,
						Code:      0,
					},
				},
			},
		}
		renderer.ForceHandleCh(ch)
	}
}

func (t *Pane) clearStatusText() {
	for i := 0; i < t.renderRect.w; i++ {
		ch := render.PositionedChar{
			Rune: ' ',
			Cursor: render.Cursor{
				X: i,
				Y: t.renderRect.h - 1,
				Style: render.Style{
					Bg: render.Color{
						ColorMode: render.ColorBit3Bright,
						Code:      2,
					},
					Fg: render.Color{
						ColorMode: render.ColorBit3Normal,
						Code:      0,
					},
				},
			},
		}
		renderer.ForceHandleCh(ch)
	}
}

func (t *Pane) kill() {
	t.vterm.Kill()
	t.shell.Kill()
}

func (t *Pane) setPause(pause bool) {
	t.vterm.ChangePause <- pause
}

func (t *Pane) serialize() string {
	out := fmt.Sprintf("Term[%dx%d]", t.renderRect.w, t.renderRect.h)
	if t.selected {
		return out + "*"
	}
	return out
}

func (t *Pane) simplify() {}

func (t *Pane) setRenderRect(x, y, w, h int) {
	t.renderRect = Rect{x, y, w, h}

	if !t.vterm.IsPaused {
		t.vterm.Reshape(x, y, w, h)
		t.vterm.RedrawWindow()
	}

	t.shell.resize(w, h)

	t.softRefresh()
}

func (t *Pane) getRenderRect() Rect {
	return t.renderRect
}

func (t *Pane) softRefresh() {
	// only selected Panes get the special highlight color
	if t.selected {
		drawSelectionBorder(t.renderRect)
	}
}
