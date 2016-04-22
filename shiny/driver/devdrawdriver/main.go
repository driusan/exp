// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"fmt"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	//	"golang.org/x/mobile/event/lifecycle"
)

// Main spawns 2 goroutines to make blocking read from /dev
// interfaces, one for the mouse and one for the keyboard
func Main(f func(s screen.Screen)) {
	mouseEvent := make(chan *mouse.Event)
	keyboardEvent := make(chan *key.Event)
	windowChan := make(chan *wctlEvent)
	doneChan := make(chan bool)

	s := newScreenImpl()
	go func() {
		// run the callback with the screen implementation, then send
		// a notification to break out of the infinite loop when it
		// exits
		f(s)
		doneChan <- true
	}()
	go mouseEventHandler(mouseEvent)
	go keyboardEventHandler(keyboardEvent)
	go wctlEventHandler(windowChan)

	var lastWindowEvent *wctlEvent

	for {
		select {
		//case mEv := <-mouseEvent:
		case mEv := <-mouseEvent:
			if s.w != nil {
				s.w.Queue.Send(mEv)
			}
		case kEv := <-keyboardEvent:
			if s.w != nil {
				//fmt.Printf("Queuing: %s\n", kEv)
				s.w.Queue.Send(*kEv)
			}
		case wEv := <-windowChan:
			if lastWindowEvent == nil {
				// reorder the window's coordinate system so that 0,0 is relative to the window.
				repositionWindow(s.ctl, wEv.windowDimensions)
				redrawImage2(s.ctl, wEv.windowDimensions)
				// TODO: Also send a lifecycle created event here.
			} else {
				fmt.Printf("%s", wEv)
				repositionWindow(s.ctl, wEv.windowDimensions)
				redrawImage2(s.ctl, wEv.windowDimensions)
				//repositionWindow(s.ctl, wEv.windowDimensions)
				//redrawImage2(s.ctl, wEv.windowDimensions)
				//(ctrl *DrawCtrler, r image.Rectangle) {
				//repositionWindow(s.ctl, wEv.windowDimensions)
				// TODO: check if dimensions changed and send a resize event
				// TODO: Check if active or current changed and send an appropriate event
			}
			lastWindowEvent = wEv
			if s.w != nil {
				//fmt.Printf("Queuing: %s\n", wEv)
				//s.w.Queue.Send(*wEv)
			}
			if wEv != nil {
				repositionWindow(s.ctl, wEv.windowDimensions)
				redrawImage2(s.ctl, wEv.windowDimensions)
			}
		case <-doneChan:
			return
		}
	}
}
