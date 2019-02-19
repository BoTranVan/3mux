/*
Package vterm provides a layer of abstraction between a channel of incoming text (possibly containing ANSI escape codes, et al) and a channel of outbound Char's.

A Char is a character printed using a given cursor (which is stored alongside the Char).
*/
package vterm

import (
	"github.com/aaronduino/i3-tmux/render"
)

// ScrollingRegion holds the state for an ANSI scrolling region
type ScrollingRegion struct {
	top    int
	bottom int
}

/*
VTerm acts as a virtual terminal emulator between a shell and the host terminal emulator

It both transforms an inbound stream of bytes into Char's and provides the option of dumping all the Char's that need to be rendered to display the currently visible terminal window from scratch.
*/
type VTerm struct {
	x, y, w, h int

	// visible screen; char cursor coords are ignored
	screen [][]render.Char

	screenOld [][]render.Char

	// scrollback[0] is the line farthest from the screen
	scrollback    [][]render.Char // disabled when using alt screen; char cursor coords are ignored
	scrollbackPos int             // scrollbackPos is the number of lines of scrollback visible

	usingAltScreen bool
	screenBackup   [][]render.Char

	NeedsRedraw bool

	startTime           int64
	shutdown            chan bool
	shellByteCounter    *uint64
	internalByteCounter uint64
	usingSlowRefresh    bool

	Cursor render.Cursor

	renderer *render.Renderer

	blankLine []render.Char

	// parentSetCursor sets physical host's cursor taking the pane location into account
	parentSetCursor func(x, y int)

	in  <-chan rune
	out chan<- render.PositionedChar

	storedCursorX, storedCursorY int

	scrollingRegion ScrollingRegion
}

// NewVTerm returns a VTerm ready to be used by its exported methods
func NewVTerm(shellByteCounter *uint64, shutdown chan bool, startTime int64, renderer *render.Renderer, parentSetCursor func(x, y int), in <-chan rune, out chan<- render.PositionedChar) *VTerm {
	w := 10
	h := 10

	screen := [][]render.Char{}
	for j := 0; j < h; j++ {
		row := []render.Char{}
		for i := 0; i < w; i++ {
			row = append(row, render.Char{
				Rune:  ' ',
				Style: render.Style{},
			})
		}
		screen = append(screen, row)
	}

	return &VTerm{
		x: 0, y: 0,
		w:                w,
		h:                h,
		blankLine:        []render.Char{},
		screen:           screen,
		screenOld:        screen,
		scrollback:       [][]render.Char{},
		usingAltScreen:   false,
		Cursor:           render.Cursor{},
		in:               in,
		out:              out,
		startTime:        startTime,
		shutdown:         shutdown,
		shellByteCounter: shellByteCounter,
		usingSlowRefresh: false,
		renderer:         renderer,
		parentSetCursor:  parentSetCursor,
		scrollingRegion:  ScrollingRegion{top: 0, bottom: h - 1},
		NeedsRedraw:      false,
	}
}

// Kill safely shuts down all vterm processes for the instance
func (v *VTerm) Kill() {
}

// Reshape safely updates a VTerm's width & height
func (v *VTerm) Reshape(x, y, w, h int) {
	blankLine := []render.Char{}
	for i := 0; i < w; i++ {
		blankLine = append(blankLine, render.Char{Rune: ' ', Style: render.Style{}})
	}

	v.blankLine = blankLine

	v.x = x
	v.y = y

	for y := 0; y <= h; y++ {
		if y >= len(v.screen) {
			v.screen = append(v.screen, []render.Char{})
		}

		for x := 0; x <= w; x++ {
			if x >= len(v.screen[y]) {
				v.screen[y] = append(v.screen[y], render.Char{Rune: ' ', Style: render.Style{}})
			}
		}
	}

	for y := 0; y <= h; y++ {
		if y >= len(v.screenOld) {
			v.screenOld = append(v.screenOld, []render.Char{})
		}

		for x := 0; x <= w; x++ {
			if x >= len(v.screenOld[y]) {
				v.screenOld[y] = append(v.screenOld[y], render.Char{Rune: ' ', Style: render.Style{}})
			}
		}
	}

	// if h > len(v.screen) { // move lines from scrollback
	// 	linesToAdd := h - len(v.screen)
	// 	scrollbackLinesToAdd := linesToAdd
	// 	if scrollbackLinesToAdd > len(v.scrollback) {
	// 		scrollbackLinesToAdd = len(v.scrollback)
	// 	}

	// 	v.screen = append(v.scrollback[len(v.scrollback)-scrollbackLinesToAdd:], v.screen...)
	// 	v.screen = append(v.screen, make([][]Char, linesToAdd-scrollbackLinesToAdd)...)
	// 	v.scrollback = v.scrollback[:len(v.scrollback)-scrollbackLinesToAdd]
	// } else if h < len(v.screen)-1 { // move lines to scrollback
	// 	linesToMove := len(v.screen) - h

	// 	v.scrollback = append(v.scrollback, v.screen[:linesToMove]...)
	// 	// v.debug(strconv.Itoa(linesToMove))
	// 	// fmt.Fprintln(os.Stdout, strconv.Itoa(len(v.screen)-linesToMove))
	// 	if linesToMove < len(v.screen) {
	// 		v.screen = v.screen[linesToMove:]
	// 	}
	// }

	if v.scrollingRegion.top == 0 && v.scrollingRegion.bottom == v.h-1 {
		v.scrollingRegion.bottom = h - 1
	}

	v.w = w
	v.h = h

	v.RedrawWindow()
}
