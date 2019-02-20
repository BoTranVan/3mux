package render

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Renderer is our simplified implemention of ncurses
type Renderer struct {
	w, h int

	currentScreen [][]Char
	pendingScreen [][]Char

	highlights [][]bool

	drawingCursor Cursor
	writingMutex  *sync.Mutex

	restingCursor Cursor

	// RenderQueue is how requests to change the framebuffer are made
	RenderQueue chan PositionedChar
}

// A PositionedChar is a char with a specific location on the screen
type PositionedChar struct {
	Rune rune
	Cursor
}

// A Char is a rune with a visual style associated with it
type Char struct {
	Rune rune
	Style
}

// NewRenderer returns an initialized Renderer
func NewRenderer() *Renderer {
	return &Renderer{
		currentScreen: [][]Char{},
		pendingScreen: [][]Char{},
		writingMutex:  &sync.Mutex{},
		RenderQueue:   make(chan PositionedChar, 100000),
	}
}

// Resize changes the size of the framebuffer to match the host terminal size
func (r *Renderer) Resize(w, h int) {
	for y := 0; y <= h; y++ {
		if y >= len(r.pendingScreen) {
			r.pendingScreen = append(r.pendingScreen, []Char{})
		}

		for x := 0; x <= w; x++ {
			if x >= len(r.pendingScreen[y]) {
				r.pendingScreen[y] = append(r.pendingScreen[y], Char{Rune: ' ', Style: Style{}})
			}
		}
	}

	for y := 0; y <= h; y++ {
		if y >= len(r.currentScreen) {
			r.currentScreen = append(r.currentScreen, []Char{})
		}

		for x := 0; x <= w; x++ {
			if x >= len(r.currentScreen[y]) {
				r.currentScreen[y] = append(r.currentScreen[y], Char{Rune: ' ', Style: Style{}})
			}
		}
	}

	for y := 0; y <= h; y++ {
		if y >= len(r.highlights) {
			r.highlights = append(r.highlights, []bool{})
		}

		for x := 0; x <= w; x++ {
			if x >= len(r.highlights[y]) {
				r.highlights[y] = append(r.highlights[y], false)
			}
		}
	}

	r.w = w
	r.h = h
}

// HandleCh places a PositionedChar in the pending screen buffer
func (r *Renderer) HandleCh(ch PositionedChar) {
	r.writingMutex.Lock()
	if ch.Rune == 0 {
		ch.Rune = ' '
	}

	r.pendingScreen[ch.Y][ch.X] = Char{
		Rune:  ch.Rune,
		Style: ch.Cursor.Style,
	}
	r.writingMutex.Unlock()
}

// ListenToQueue is a blocking function that processes data sent to the RenderQueue
func (r *Renderer) ListenToQueue() {
	fmt.Print("\033[2J") // clear screen

	for {
		fmt.Print("\033[?25l") // hide cursor

		var diff strings.Builder
		for y := 0; y < r.h; y++ {
			for x := 0; x < r.w; x++ {
				r.writingMutex.Lock()
				current := r.currentScreen[y][x]
				pending := r.pendingScreen[y][x]
				// FIXME: should only update changed portions of the screen
				if current != pending || true {
					r.currentScreen[y][x] = pending

					newCursor := Cursor{
						X: x, Y: y, Style: pending.Style,
					}

					if r.highlights[y][x] {
						newCursor.Style.Bg = Color{
							ColorMode: ColorBit3Bright,
							Code:      6,
						}
					}

					delta := deltaMarkup(r.drawingCursor, newCursor)
					diff.WriteString(delta)
					diff.WriteString(string(pending.Rune))
					newCursor.X++
					r.drawingCursor = newCursor
				}
				r.writingMutex.Unlock()
			}
		}

		fmt.Print(diff.String())

		delta := deltaMarkup(r.drawingCursor, r.restingCursor)
		fmt.Print(delta)
		r.drawingCursor = r.restingCursor

		fmt.Print("\033[?25h") // show cursor

		time.Sleep(time.Millisecond * 25)
	}
}

// SetCursor sets the position of the physical cursor
func (r *Renderer) SetCursor(x, y int) {
	r.restingCursor = Cursor{
		X: x, Y: y, Style: r.drawingCursor.Style,
	}
}

// Debug prints the given text to the status bar
func (r *Renderer) Debug(s string) {
	for i, ch := range s {
		r.HandleCh(PositionedChar{
			Rune: rune(ch),
			Cursor: Cursor{
				X: i, Y: r.h - 1,
				Style: Style{},
			}})
	}
}

// Highlight visually highlights the selected chars
func (r *Renderer) Highlight(x, y int) {
	r.highlights[y][x] = true
}

// UnhighlightAll removes the highlight from all highlighted characters
func (r *Renderer) UnhighlightAll() {
	for y := range r.highlights {
		for x := range r.highlights[y] {
			r.highlights[y][x] = false
		}
	}
}
