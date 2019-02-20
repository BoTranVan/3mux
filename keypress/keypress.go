/*
Package keypress is a library for advanced keypress detection and parsing.
*/
package keypress

import (
	"fmt"
	"log"
	"strings"
	"unicode"

	term "github.com/nsf/termbox-go"
)

var callback func(name string, raw []byte)

// Listen is a blocking function that indefinitely listens for keypresses.
// When it detects a keypress, it passes on to the callback a human-readable interpretation of the event (e.g. Alt+Shift+Up) along with the raw string of text received by the terminal.
func Listen(c func(name string, raw []byte)) {
	callback = c

	err := term.Init()
	if err != nil {
		log.Fatal(err)
	}
	defer term.Close()

	fmt.Print("\033[?25h\033[?12h") // show cursor, make it blink

	for {
		data := make([]byte, 16)
		ev := term.PollRawEvent(data)

		trimmedData := data[:ev.N]

		handle := func(name string) {
			callback(name, trimmedData)
		}

		switch data[0] {
		case 13:
			handle("Enter")
		case 195: // Alt
			letter := rune(data[1] - 128 + 64)
			if unicode.IsUpper(letter) {
				handle("Alt+Shift+" + string(unicode.ToUpper(letter)))
			} else {
				handle("Alt+" + string(unicode.ToUpper(letter)))
			}
		case 27: // Escape code
			if ev.N == 1 { // Lone Esc Key
				handle("Esc")
			}

			arrowNames := map[byte]string{
				65: "Up",
				66: "Down",
				67: "Right",
				68: "Left",
			}

			switch data[1] {
			case 79:
				direction := arrowNames[data[2]]
				if ev.N == 15 {
					handle("Scroll " + direction)
				} else {
					handle(direction)
				}
			case 91:
				switch data[2] {
				case 51:
					code := string(trimmedData[2:])
					code = strings.TrimSuffix(code, "M") // NOTE: are there other codes we are forgetting about?
					pieces := strings.Split(code, ";")
					switch pieces[0] {
					case "2":
						handle("Mouse Down")
					case "5":
						handle("Mouse Up")
					case "32":
						handle("Start Selection")
					case "35":
						handle("End Selection")
					}
				case 57: // Scrolling
					switch data[3] {
					case 54:
						handle("Scroll Up")
					case 55:
						handle("Scroll Down")
					}
				default:
					arrow := arrowNames[data[5]]
					switch data[4] {
					case 50:
						handle("Shift+" + arrow)
					case 51:
						handle("Alt+" + arrow)
					case 52:
						handle("Alt+Shift+" + arrow)
					case 53:
						handle("Ctrl+" + arrow)
					}
				}
			default:
				letter := rune(data[1])
				if unicode.IsUpper(letter) {
					handle("Alt+Shift+" + string(letter))
				} else {
					handle("Alt+" + string(unicode.ToUpper(letter)))
				}
			}
		default:
			if ev.N == 1 {
				if data[0] <= 26 { // Ctrl
					letter := string('A' + data[0] - 1)
					if letter == "Q" { // exit upon Ctrl+Q
						return
					}
					handle("Ctrl+" + letter)
				} else {
					letter := string(data[0])
					handle(letter)
				}
			}
		}

		// // debugging code
		// log.Println(ev)
		// log.Print(data)
		// str := ""
		// for _, b := range data {
		// 	str += string(b)
		// }
		// log.Println(str)
		// log.Println()
	}
}
