// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"fmt"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"log"
	"os"
	"strconv"
	"strings"
)

// ButtonMask represents the Plan9 button masks as read from /dev/mouse.
// Plan9 uses a bitmask of the buttons that are pressed, while mouse.Event
// expects one event per action and a direction. We need to convert the
// bitmask to an event every time we receive a message by calculating
// the direction based on the previous button pressed.
type ButtonMask int

const (
	MouseButtonLeft   = ButtonMask(1)
	MouseButtonMiddle = ButtonMask(2)
	MouseButtonRight  = ButtonMask(4)
	MouseScrollUp     = ButtonMask(8)
	MouseScrollDown   = ButtonMask(16)
)

// mouseEventHandler runs in a go routine to continuously make (blocking)
// reads from /dev/mouse and converts them to mouse.Event messages which
// are passed along the notifier channel to be added to the shiny event
// queue.
// BUG: This doesn't handle the 'r' message type yet.
func mouseEventHandler(notifier chan *mouse.Event, s *screenImpl) {
	mouseEvent, err := os.Open("/dev/mouse")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open mouse driver.\n")
		return
	}
	defer mouseEvent.Close()

	mouseMessage := make([]byte, 100)
	// used to determine if it's an up or a down direction
	var previousEvent mouse.Event
	for {
		_, err := mouseEvent.Read(mouseMessage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unexpected data from the mouse.\n")
			continue

		}
		switch mouseMessage[0] {
		case 'r':
			// Reread the window size the same way that happens on startup.
			// This is more reliable than the 'r' message, the format of which
			// isn't documented.
			windowSize, err := readWctl()
			if err != nil {
				log.Fatalf("read current window size: %v\n", err)
			}

			s.windowFrame = windowSize
			repositionWindow(s, s.windowFrame)
			if s.w != nil {
				sz := s.windowFrame.Size()
				// tell the window it's current size before doing anything.
				s.w.Queue.Send(size.Event{WidthPx: sz.X, HeightPx: sz.Y})
				// and after it knows the size, tell the program using it to paint.
				s.w.Queue.Send(paint.Event{})
			}

		case 'm':
			if mouseMessage[12] != ' ' {
				fmt.Fprintf(os.Stderr, "Unhandled data from /dev/mouse: %s\n", mouseMessage)
			}

			// /dev/mouse prints an ASCII integer number, but x/mobile/event/mouse.Event
			// expects a float32, so we just parse it as a float32.
			x, err := strconv.ParseFloat(strings.TrimSpace(string(mouseMessage[1:12])), 32)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unexpected data from the mouse. Could not parse X coordinate.\n")
				continue
			}
			y, err := strconv.ParseFloat(strings.TrimSpace(string(mouseMessage[13:24])), 32)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unexpected data from the mouse. Could not parse Y coordinate.\n")
				continue
			}

			btnMaskInt, err := strconv.Atoi(strings.TrimSpace(string(mouseMessage[25:36])))
			buttons := ButtonMask(btnMaskInt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unexpected data from the mouse. Could not parse button mask.\n")
				continue
			}

			var btn mouse.Button

			// Convert the Plan9 button mask to a event.Mouse button.
			switch {
			case (buttons & MouseButtonLeft) != 0:
				btn = mouse.ButtonLeft
			case (buttons & MouseButtonMiddle) != 0:
				btn = mouse.ButtonMiddle
			case (buttons & MouseButtonRight) != 0:
				btn = mouse.ButtonRight
			case (buttons & MouseScrollUp) != 0:
				btn = mouse.ButtonWheelUp
			case (buttons & MouseScrollDown) != 0:
				btn = mouse.ButtonWheelDown
			default:
				btn = mouse.ButtonNone
			}

			var dir mouse.Direction = mouseDirection(previousEvent.Button, btn)
			newEvent := mouse.Event{
				X:         float32(x),
				Y:         float32(y),
				Button:    btn,
				Modifiers: currentModifiers,
				Direction: dir,
			}
			notifier <- &newEvent
			previousEvent = newEvent
		default:
			fmt.Fprintf(os.Stderr, "Unhandled mouse event: %s\n", mouseMessage)
		}
	}
}

// mouseDirection calculates the direction of the button press by comparing
// the previous button to this one.
func mouseDirection(prev mouse.Button, cur mouse.Button) mouse.Direction {
	if prev == cur {
		return mouse.DirNone
	} else {
		if prev == mouse.ButtonNone {
			return mouse.DirPress
		}
		if cur == mouse.ButtonNone {
			return mouse.DirRelease
		}
	}
	return mouse.DirNone
}
