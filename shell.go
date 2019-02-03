package main

import (
	"log"
	"math/rand"
	"os"
	"os/exec"

	"github.com/aaronduino/i3-tmux/vterm"
	"github.com/kr/pty"
)

// A Term is the fundamental leaf unit of screen space
type Term struct {
	id int

	selected bool

	renderRect Rect

	ptmx *os.File
	cmd  *exec.Cmd

	vterm *vterm.VTerm

	vtermIn         chan<- rune
	vtermOut        <-chan vterm.Char
	vtermOperations <-chan vterm.Operation
}

func newTerm(selected bool) *Term {
	// Create arbitrary command.
	c := exec.Command("zsh")

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		log.Fatal(err)
	}

	vtermIn := make(chan rune, 32)
	vtermOut := make(chan vterm.Char, 32)
	vtermOperations := make(chan vterm.Operation, 32)

	vt := vterm.NewVTerm(vtermIn, vtermOut, vtermOperations)

	t := &Term{
		id:       rand.Intn(10),
		selected: selected,
		ptmx:     ptmx,
		cmd:      c,

		vterm:           vt,
		vtermIn:         vtermIn,
		vtermOut:        vtermOut,
		vtermOperations: vtermOperations,
	}

	go (func() {
		defer func() {
			if r := recover(); r != nil {
				if r != "send on closed channel" {
					panic(r)
				}
			}
		}()

		for {
			b := make([]byte, 1)
			_, err := ptmx.Read(b)
			if err != nil {
				return
			}
			r := rune(b[0])
			if r == '\t' { // FIXME
				t.vtermIn <- ' '
				t.vtermIn <- ' '
			} else {
				t.vtermIn <- r
			}
		}
	})()

	go (func() {
		for {
			char := <-vtermOut
			if char.Cursor.X > t.renderRect.w-1 {
				continue
			}
			if char.Cursor.Y > t.renderRect.h-1 {
				continue
			}
			char.Cursor.X += t.renderRect.x
			char.Cursor.Y += t.renderRect.y
			globalCharAggregate <- char
		}
	})()

	go (func() {
		for {
			oper := <-vtermOperations
			text := oper.Serialize(t.renderRect.x, t.renderRect.y, t.renderRect.w, t.renderRect.h, globalCursor)
			globalRawAggregate <- text
		}
	})()

	go vt.ProcessStream()

	return t
}

func (t *Term) kill() {
	t.vterm.StopBlinker()

	close(t.vtermIn)

	err := t.ptmx.Close()
	if err != nil {
		log.Fatal("TERM_CLOSE", err)
	}

	err = t.cmd.Process.Kill()
	if err != nil {
		log.Fatal("TERM_KILL", err)
	}
}

func (t *Term) handleStdin(text string) {
	_, err := t.ptmx.Write([]byte(text))
	if err != nil {
		log.Fatal(err)
	}
}
