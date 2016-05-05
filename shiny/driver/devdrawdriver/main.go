// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"fmt"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"os"
)

// Main spawns 2 goroutines to make blocking reads from /dev
// interfaces, one for the mouse and one for the keyboard
// Window events such as resize and move come in over the mouse
// channel.
func Main(f func(s screen.Screen)) {
	mouseEvent := make(chan *mouse.Event)
	keyboardEvent := make(chan *key.Event)
	doneChan := make(chan bool)

	s := newScreenImpl()
	// read the current window size that will be drawn into from
	// /dev/wctl
	windowSize, err := readWctl()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read current window size.\n")
		return
	}
	s.windowFrame = windowSize
	go func() {
		// run the callback with the screen implementation, then send
		// a notification to break out of the infinite loop when it
		// exits
		f(s)
		doneChan <- true
	}()

	go mouseEventHandler(mouseEvent)
	go keyboardEventHandler(keyboardEvent)
	for {
		select {
		case mEv := <-mouseEvent:
			if s.w != nil {
				s.w.Queue.Send(mEv)
			}
		case kEv := <-keyboardEvent:
			if s.w != nil {
				s.w.Queue.Send(*kEv)
			}
		case <-doneChan:
			return
		}
	}
}
