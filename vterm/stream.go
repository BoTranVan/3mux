package vterm

import (
	"fmt"
	"log"
	"unicode"
	"unicode/utf8"

	"github.com/aaronduino/i3-tmux/cursor"
)

// ProcessStream processes and transforms a process' stdout, turning it into a stream of Char's to be sent to the rendering scheduler
// This includes translating ANSI Cursor coordinates and maintaining a scrolling buffer
func (v *VTerm) ProcessStream() {
	for {
		next, ok := <-v.Stream
		if !ok {
			return
		}

		// <-time.NewTimer(time.Second / 32).C

		if next > 127 {
			value := []byte{byte(next)}

			leadingHex := next >> 4
			switch leadingHex {
			case 12: // 1100
				value = append(value, byte(<-v.Stream))
			case 14: // 1110
				value = append(value, byte(<-v.Stream))
				value = append(value, byte(<-v.Stream))
			case 15: // 1111
				value = append(value, byte(<-v.Stream))
				value = append(value, byte(<-v.Stream))
				value = append(value, byte(<-v.Stream))
			}

			next, _ = utf8.DecodeRune(value)
		}

		switch next {
		case '\x1b':
			v.handleEscapeCode()
		case 8:
			v.Cursor.X--
			if v.Cursor.X < 0 {
				v.Cursor.X = 0
			}
			v.updateCursor()
		case '\n':
			if v.Cursor.Y == v.scrollingRegion.bottom {
				v.scrollDown(1)

				// // disable scrollback if using alt screen
				// if !v.usingAltScreen && len(v.screen) > v.scrollingRegion.top {
				// 	// v.scrollback = append(v.scrollback, v.screen[len(v.screen)-1:]...)
				// 	v.scrollback = append(v.scrollback, v.screen[v.scrollingRegion.top])
				// }

				// // v.screen = append(v.screen[:len(v.screen)-1], []Char{})
				// v.screen = append(append(append(
				// 	v.screen[:v.scrollingRegion.top],
				// 	v.screen[v.scrollingRegion.top+1:v.scrollingRegion.bottom+1]...),
				// 	[]Char{}),
				// 	v.screen[v.scrollingRegion.bottom+1:]...)

				// v.oper <- ScrollDown{}

				// v.RedrawWindow()
			} else {
				v.Cursor.Y++
			}
			v.updateCursor()
		case '\r':
			v.Cursor.X = 0
			v.updateCursor()
		default:
			if unicode.IsPrint(next) {
				if v.Cursor.X < 0 {
					v.Cursor.X = 0
				}
				if v.Cursor.Y < 0 {
					v.Cursor.Y = 0
				}

				char := Char{
					Rune:   next,
					Cursor: v.Cursor,
				}

				if len(v.screen)-1 < v.Cursor.Y {
					for i := len(v.screen); i < v.Cursor.Y+1; i++ {
						v.screen = append(v.screen, []Char{Char{
							Rune: 0,
						}})
					}
				}
				if len(v.screen[v.Cursor.Y])-1 < v.Cursor.X {
					for i := len(v.screen[v.Cursor.Y]); i < v.Cursor.X+1; i++ {
						v.screen[v.Cursor.Y] = append(v.screen[v.Cursor.Y], Char{
							Rune: 0,
						})
					}
				}

				v.screen[v.Cursor.Y][v.Cursor.X] = char

				if v.Cursor.X < v.w && v.Cursor.Y < v.h {
					char.Cursor.X = v.Cursor.X
					char.Cursor.Y = v.Cursor.Y
					v.out <- char
				}

				v.Cursor.X++
				v.updateCursor()
			} else {
				v.debug(fmt.Sprintf("%x    ", next))
			}
		}
	}
}

func (v *VTerm) handleEscapeCode() {
	next, ok := <-v.Stream
	if !ok {
		log.Fatal("not ok")
		return
	}

	switch next {
	case '[':
		v.handleCSISequence()
	case '(': // Character set
		<-v.Stream
		// TODO
	default:
		v.debug("ESC Code: " + string(next))
	}
}

func (v *VTerm) handleCSISequence() {
	privateSequence := false

	// <-time.NewTimer(time.Second / 2).C

	parameterCode := ""
	for {
		next, ok := <-v.Stream
		if !ok {
			return
		}

		if unicode.IsDigit(next) || next == ';' || next == ' ' {
			parameterCode += string(next)
		} else if next == '?' {
			privateSequence = true
		} else if privateSequence {
			switch next {
			case 'h':
				switch parameterCode {
				case "1": // application arrow keys (DECCKM)
				case "7": // Auto-wrap Mode (DECAWM)
				case "12": // start blinking Cursor
				case "25": // show Cursor
					// v.StartBlinker()
				case "1049", "1047", "47": // switch to alt screen buffer
					if !v.usingAltScreen {
						v.screenBackup = v.screen
					}
				case "2004": // enable bracketed paste mode
				default:
					v.debug("CSI Private H Code: " + parameterCode + string(next))
				}
			case 'l':
				switch parameterCode {
				case "1": // Normal Cursor keys (DECCKM)
				case "7": // No Auto-wrap Mode (DECAWM)
				case "12": // stop blinking Cursor
				case "25": // hide Cursor
					// v.StopBlinker()
				case "1049", "1047", "47": // switch to normal screen buffer
					if v.usingAltScreen {
						v.screen = v.screenBackup
					}
				case "2004": // disable bracketed paste mode
					// TODO
				default:
					v.debug("CSI Private L Code: " + parameterCode + string(next))
				}
			default:
				v.debug("CSI Private Code: " + parameterCode + string(next))
			}
			return
		} else {
			// if next != 'H' && next != 'C' && next != 'G' && next != 'm' {
			// 	v.debug(string(next))
			// }
			// v.debug(string(next))
			switch next {
			case 'A': // Cursor Up
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.Y -= seq[0]
				if v.Cursor.Y < 0 {
					v.Cursor.Y = 0
				}
				v.updateCursor()
			case 'B': // Cursor Down
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.Y += seq[0]
				v.updateCursor()
			case 'C': // Cursor Right
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.X += seq[0]
				v.updateCursor()
			case 'D': // Cursor Left
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.X -= seq[0]
				if v.Cursor.X < 0 {
					v.Cursor.X = 0
				}
				v.updateCursor()
			case 'd': // Vertical Line Position Absolute (VPA)
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.Y = seq[0] - 1
				v.updateCursor()
			case 'E': // Cursor Next Line
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.Y += seq[0]
				v.Cursor.X = 0
				v.updateCursor()
			case 'F': // Cursor Previous Line
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.Y -= seq[0]
				v.Cursor.X = 0
				if v.Cursor.Y < 0 {
					v.Cursor.Y = 0
				}
				v.updateCursor()
			case 'G': // Cursor Horizontal Absolute
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.X = seq[0] - 1
			case 'H', 'f': // Cursor Position
				seq := parseSemicolonNumSeq(parameterCode, 1)
				if parameterCode == "" {
					v.Cursor.X = 0
					v.Cursor.Y = 0
				} else {
					v.Cursor.Y = seq[0] - 1
					if v.Cursor.Y < 0 {
						v.Cursor.Y = 0
					}
					if len(seq) > 1 {
						v.Cursor.X = seq[1] - 1
						if v.Cursor.X < 0 {
							v.Cursor.X = 0
						}
					}
				}
				v.updateCursor()
			case 'J': // Erase in Display
				seq := parseSemicolonNumSeq(parameterCode, 0)
				switch seq[0] {
				case 0: // clear from Cursor to end of screen
					for i := v.Cursor.X; i < len(v.screen[v.Cursor.Y]); i++ {
						v.screen[v.Cursor.Y][i].Rune = 0
						char := v.screen[v.Cursor.Y][i]
						char.Rune = ' '
						char.Cursor = cursor.Cursor{X: i, Y: v.Cursor.Y}
						v.out <- char
					}
					if v.Cursor.Y+1 < len(v.screen) {
						for j := v.Cursor.Y; j < len(v.screen); j++ {
							for i := 0; i < len(v.screen[j]); i++ {
								v.screen[j][i].Rune = 0
								char := v.screen[j][i]
								char.Rune = ' '
								char.Cursor = cursor.Cursor{X: i, Y: j}
								v.out <- char
							}
						}
					}
					// v.RedrawWindow()
				case 1: // clear from Cursor to beginning of screen
					for j := 0; j < v.Cursor.Y; j++ {
						for i := 0; i < len(v.screen[j]); j++ {
							v.screen[j][i].Rune = 0
							char := v.screen[j][i]
							char.Rune = ' '
							char.Cursor = cursor.Cursor{X: i, Y: j}
							v.out <- char
						}
					}
					for i := 0; i < v.Cursor.X; i++ {
						v.screen[v.Cursor.Y][i].Rune = 0
						char := v.screen[v.Cursor.Y][i]
						char.Rune = ' '
						char.Cursor = cursor.Cursor{X: i, Y: v.Cursor.Y}
						v.out <- char
					}
				case 2: // clear entire screen (and move Cursor to top left?)
					for i := range v.screen {
						for j := range v.screen[i] {
							v.screen[i][j].Rune = ' '
							char := v.screen[i][j]
							char.Rune = ' '
							char.Cursor = cursor.Cursor{X: j, Y: i}
							v.out <- char
						}
					}
					v.Cursor.X = 0
					v.Cursor.Y = 0
					v.RedrawWindow()
				case 3: // clear entire screen and delete all lines saved in scrollback buffer
					for i := range v.screen {
						for j := range v.screen[i] {
							v.screen[i][j].Rune = ' '
						}
					}
					v.Cursor.X = 0
					v.Cursor.Y = 0
					v.scrollback = [][]Char{}
					v.RedrawWindow()
				}
			case 'K': // Erase in Line
				seq := parseSemicolonNumSeq(parameterCode, 0)
				switch seq[0] {
				case 0: // clear from Cursor to end of line
					for i := v.Cursor.X; i < len(v.screen[v.Cursor.Y]); i++ { // FIXME: sometimes crashes
						v.screen[v.Cursor.Y][i].Rune = 0
					}
				case 1: // clear from Cursor to beginning of line
					for i := 0; i < v.Cursor.X; i++ {
						v.screen[v.Cursor.Y][i].Rune = 0
					}
				case 2: // clear entire line; Cursor position remains the same
					for i := 0; i < len(v.screen[v.Cursor.Y]); i++ {
						v.screen[v.Cursor.Y][i].Rune = 0
					}
				}
				v.RedrawWindow()
			case 'r': // Set Scrolling Region
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.scrollingRegion.top = seq[0] - 1
				if len(seq) > 1 {
					v.scrollingRegion.bottom = seq[1] - 1
				} else {
					v.scrollingRegion.bottom = v.h + 1
				}
				v.Cursor.X = 0
				v.Cursor.Y = 0
			case 'S': // Scroll Up; new lines added to bottom
				seq := parseSemicolonNumSeq(parameterCode, 1)
				numLines := seq[0]
				v.scrollDown(numLines)
			case 'T': // Scroll Down; new lines added to top
				seq := parseSemicolonNumSeq(parameterCode, 1)
				numLines := seq[0]
				// v.screen = append(v.scrollback[len(v.scrollback)-numLines:], v.screen...)
				// v.scrollback = v.scrollback[:len(v.scrollback)-numLines]

				newLines := make([][]Char, numLines)

				// v.screen = append(v.screen[:len(v.screen)-1], []Char{})
				v.screen = append(append(append(
					v.screen[:v.scrollingRegion.top],
					newLines...),
					v.screen[v.scrollingRegion.top:v.scrollingRegion.bottom-numLines]...),
					v.screen[v.scrollingRegion.bottom+1:]...)

				v.RedrawWindow()
			case 'L': // Insert Lines
				seq := parseSemicolonNumSeq(parameterCode, 1)
				v.Cursor.X = 0

				if v.Cursor.Y < v.scrollingRegion.top || v.Cursor.Y > v.scrollingRegion.bottom {
					v.debug("unhandled insert line operation")
					return
				}

				numLines := seq[0]
				newLines := make([][]Char, numLines)

				above := [][]Char{}
				if v.Cursor.Y > 0 {
					above = v.screen[:v.Cursor.Y]
				}

				v.screen = append(append(append(
					above,
					newLines...),
					v.screen[v.Cursor.Y:v.scrollingRegion.bottom-numLines+1]...),
					v.screen[v.scrollingRegion.bottom+1:]...)

				v.Cursor.Y++
				v.updateCursor()
				v.RedrawWindow()
				// v.debug("                        " + strconv.Itoa(v.Cursor.Y))
			case 'm': // Select Graphic Rendition
				v.handleSDR(parameterCode)
			case 's': // Save Cursor Position
				v.storedCursorX = v.Cursor.X
				v.storedCursorY = v.Cursor.Y
				v.updateCursor()
			case 'u': // Restore Cursor Positon
				v.Cursor.X = v.storedCursorX
				v.Cursor.Y = v.storedCursorY
				v.updateCursor()
			default:
				v.debug("CSI Code: " + string(next) + " ; " + parameterCode)
			}
			return
		}
	}
}

func (v *VTerm) handleSDR(parameterCode string) {
	seq := parseSemicolonNumSeq(parameterCode, 0)

	if parameterCode == "39;49" {
		v.Cursor.Fg.ColorMode = cursor.ColorNone
		v.Cursor.Bg.ColorMode = cursor.ColorNone
		return
	}

	c := seq[0]

	switch c {
	case 0:
		v.Cursor.Reset()
	case 1:
		v.Cursor.Bold = true
	case 2:
		v.Cursor.Faint = true
	case 3:
		v.Cursor.Italic = true
	case 4:
		v.Cursor.Underline = true
	case 5: // slow blink
	case 6: // rapid blink
	case 7: // swap foreground & background; see case 27
	case 8:
		v.Cursor.Conceal = true
	case 9:
		v.Cursor.CrossedOut = true
	case 10: // primary/default font
	case 22:
		v.Cursor.Bold = false
		v.Cursor.Faint = false
	case 23:
		v.Cursor.Italic = false
	case 24:
		v.Cursor.Underline = false
	case 25: // blink off
	case 27: // inverse off; see case 7
		// TODO
	case 28:
		v.Cursor.Conceal = false
	case 29:
		v.Cursor.CrossedOut = false
	case 38: // set foreground color
		if seq[1] == 5 {
			v.Cursor.Fg = cursor.Color{
				ColorMode: cursor.ColorBit8,
				Code:      int32(seq[2]),
			}
		} else if seq[1] == 2 {
			v.Cursor.Fg = cursor.Color{
				ColorMode: cursor.ColorBit24,
				Code:      int32(seq[2]<<16 + seq[3]<<8 + seq[4]),
			}
		}
	case 39: // default foreground color
		v.Cursor.Fg.ColorMode = cursor.ColorNone
	case 48: // set background color
		if seq[1] == 5 {
			v.Cursor.Bg = cursor.Color{
				ColorMode: cursor.ColorBit8,
				Code:      int32(seq[2]),
			}
		} else if seq[1] == 2 {
			v.Cursor.Bg = cursor.Color{
				ColorMode: cursor.ColorBit24,
				Code:      int32(seq[2]<<16 + seq[3]<<8 + seq[4]),
			}
		}
	case 49: // default background color
		v.Cursor.Bg.ColorMode = cursor.ColorNone
	default:
		if c >= 30 && c <= 37 {
			if len(seq) > 1 && seq[1] == 1 {
				v.Cursor.Fg = cursor.Color{
					ColorMode: cursor.ColorBit3Bright,
					Code:      int32(c - 30),
				}
			} else {
				v.Cursor.Fg = cursor.Color{
					ColorMode: cursor.ColorBit3Normal,
					Code:      int32(c - 30),
				}
			}
		} else if c >= 40 && c <= 47 {
			if len(seq) > 1 && seq[1] == 1 {
				v.Cursor.Bg = cursor.Color{
					ColorMode: cursor.ColorBit3Bright,
					Code:      int32(c - 40),
				}
			} else {
				v.Cursor.Bg = cursor.Color{
					ColorMode: cursor.ColorBit3Normal,
					Code:      int32(c - 40),
				}
			}
		} else if c >= 90 && c <= 97 {
			v.Cursor.Fg = cursor.Color{
				ColorMode: cursor.ColorBit3Bright,
				Code:      int32(c - 90),
			}
		} else if c >= 100 && c <= 107 {
			v.Cursor.Bg = cursor.Color{
				ColorMode: cursor.ColorBit3Bright,
				Code:      int32(c - 100),
			}
		} else {
			v.debug("SGR Code: " + string(parameterCode))
		}
	}
}
