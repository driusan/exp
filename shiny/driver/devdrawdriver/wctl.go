// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"fmt"
	"image"
	"os"
	"strings"
)

type wctlEvent struct {
	windowDimensions image.Rectangle
	visible, active  bool
}

// wctlEventHandler reads events on the current window that happened on
// /dev/wctl and converts them to events which are passed along the notification
// channel, where they'll be converted to shiny events in the main thread.
// There can be many different types of events that changes to the status
// implies that are easier to interpret on the main thread (window creation,
// resize, changing from visible to not visible, etc..)
func wctlEventHandler(notifier chan *wctlEvent) {
	ctl, err := os.OpenFile("/dev/wctl", os.O_RDWR, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current window status.\n")
		return
	}
	defer ctl.Close()

	event := make([]byte, 12*4 /* 4 numbers with whitespace padding */ +len("notvisible")+1 /* space */ +len("notcurrent")+1000)
	for {
		// the message might not include the "not" prefix, so we don't care if we under-read
		_, err := ctl.Read(event)
		if err != nil {
			panic(err)
		}
		sizes := strings.Fields(string(event))
		if len(sizes) != 6 && len(sizes) != 7 /* For some reason Go thinks that 6=7 pieces of the string. Seems to be a bug with Fields() */ {
			fmt.Fprintf(os.Stderr, "Invalid message received on /dev/wctl (%d, %s)\n", len(sizes), sizes)
			continue
		}
		fmt.Printf("%s: %s\n", event, sizes)
		notifier <- &wctlEvent{
			windowDimensions: image.Rectangle{
				Min: image.Point{strToInt(sizes[0]), strToInt(sizes[1])},
				Max: image.Point{strToInt(sizes[2]), strToInt(sizes[3])},
			},
			active:  (sizes[4] == "current"),
			visible: (sizes[5] == "visible"),
		}

	}
}
