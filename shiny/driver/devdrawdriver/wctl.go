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

// readWctl reads /dev/wctl to get the current Plan 9 window
// size. This is done once on startup to figure out the frame
// that will be used for drawing into. After that, resize
// and move events come through /dev/mouse.
func readWctl() (image.Rectangle, error) {
	ctl, err := os.OpenFile("/dev/wctl", os.O_RDWR, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current window status.\n")
		return image.ZR, err
	}
	defer ctl.Close()
	value := make([]byte, 1024) // 1024 should be enough..
	_, err = ctl.Read(value)
	if err != nil {
		return image.ZR, err
	}
	sizes := strings.Fields(string(value))
	return image.Rectangle{
		Min: image.Point{strToInt(sizes[0]), strToInt(sizes[1])},
		Max: image.Point{strToInt(sizes[2]), strToInt(sizes[3])},
	}, nil
}
