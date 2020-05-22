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
	var prevmask ButtonMask
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
			s.w.resize(windowSize)
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

			// Convert the Plan9 button mask to a event.Mouse button.
			// It would be nice if this could be a switch statement, but multiple
			// cases would potentially need to match, so instead just set a bool
			// to track if any button changed, and send a none event if it doesn't
			// get triggered.
			sentEvt := false

			// Left click
			if (buttons&MouseButtonLeft) != 0 && (prevmask&MouseButtonLeft) == 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonLeft,
					Direction: mouse.DirPress,
				}
				sentEvt = true
			}
			// Left release
			if (buttons&MouseButtonLeft) == 0 && (prevmask&MouseButtonLeft) != 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonLeft,
					Direction: mouse.DirRelease,
				}
				sentEvt = true
			}

			// Middle click
			if (buttons&MouseButtonMiddle) != 0 && (prevmask&MouseButtonMiddle) == 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonMiddle,
					Direction: mouse.DirPress,
				}
				sentEvt = true
			}
			// Middle release
			if (buttons&MouseButtonMiddle) == 0 && (prevmask&MouseButtonMiddle) != 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonMiddle,
					Direction: mouse.DirRelease,
				}
				sentEvt = true
			}

			// Right click
			if (buttons&MouseButtonRight) != 0 && (prevmask&MouseButtonRight) == 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonRight,
					Direction: mouse.DirPress,
				}
				sentEvt = true
			}
			// Right release
			if (buttons&MouseButtonRight) == 0 && (prevmask&MouseButtonRight) != 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonRight,
					Direction: mouse.DirRelease,
				}
				sentEvt = true
			}

			// WheelUp start
			if (buttons&MouseScrollUp) != 0 && (prevmask&MouseScrollUp) == 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonWheelUp,
					Direction: mouse.DirPress,
				}
				sentEvt = true
			}
			// WheelUp end
			if (buttons&MouseScrollUp) == 0 && (prevmask&MouseScrollUp) != 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonWheelUp,
					Direction: mouse.DirRelease,
				}
				sentEvt = true
			}
			// WheelDown start
			if (buttons&MouseScrollDown) != 0 && (prevmask&MouseScrollDown) == 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonWheelDown,
					Direction: mouse.DirPress,
				}
				sentEvt = true
			}
			// WheelDown end
			if (buttons&MouseScrollDown) == 0 && (prevmask&MouseScrollDown) != 0 {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonWheelDown,
					Direction: mouse.DirRelease,
				}
				sentEvt = true
			}

			// Default. The mouse moved without any buttons changing state.
			if sentEvt == false {
				notifier <- &mouse.Event{
					X:         float32(x),
					Y:         float32(y),
					Button:    mouse.ButtonNone,
					Direction: mouse.DirNone,
				}
			}

			prevmask = buttons
		default:
			fmt.Fprintf(os.Stderr, "Unhandled mouse event: %s\n", mouseMessage)
		}
	}
}
