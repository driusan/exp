// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package devdrawdriver

import (
	"golang.org/x/exp/shiny/screen"
	"image"
	"image/color"
	"image/draw"
)

type textureImpl struct {
	// use s.n to get the connection id to use in /dev/draw/n/
	s         *screen.Screen
	buffer    *bufferImpl
	textureId int
}

func (t *textureImpl) Bounds() image.Rectangle {
	if t.buffer != nil {
		return t.buffer.Bounds()
	}
	return image.ZR
}
func (t *textureImpl) Size() image.Point {
	if t.buffer != nil {
		return t.buffer.Size()
	}
	return image.ZP
}
func (t *textureImpl) Release() {
	if t == nil {
		return
	}
	// BUG: this should write a free message to /dev/draw/n/ctl
	// f -> textureId
	if t.buffer != nil {
		t.buffer.Release()
	}
}

func (t *textureImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	// write to /dev/draw/(s.n)/data ->
	// b id[4] screenid[4] refresh[1] chan[4] repl[1] r[4*4] clipr[4*4] color[4]
	// best guess:
	// bufferId -> to add to screen.Buffer, calculate from maxSoFar
	// b
	//    bufferId
	//    s.n(?)
	//    refresh[1] -> Refbackup, look up code.
	//    chan[4] -> binary version of r8g8b8a8 (?) -- "see image(6)"
	//    repl[1] -> ??? (see draw(2))
	//    r[4*4]-> -> calculate by aligning sr.Min with dp
	//    clipr[4*4] -> same value as r
	//    color[4] -> 0xffffffff // white background?
	//
	// maybe this should really be the d operation and the b should be in buffer?
	// d dstid[4] srcid[4] maskid[4] dstr[4*4] srcp[2*4] maskp[2*4]
	//   =>
	// d t.textureId
	//   src.BufferId
	//   maskid = nil (verify in man page that it works the same way as in Go)
	//   dstr[4*4] -> calc from dp and size of sr
	//   srcp -> sr.Min
	//   maskp -> nil ??

}

func (t *textureImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {
	// first write to /dev/draw/(s.n)/data ->
	//   O op[1] (look up codes for op)
	// according to draw(2)
	// SOverD        = SinD|SoudD|DoutS => 8|1|2
	// S (=src?)     = SinD|SoutD       => 8|2

	// then look up if there's a srcid for the colour in a map, if not create it with
	// /dev/data/n/data -> b message of same size as texture, and cache it
	// in a map.

	// then write to /dev/draw/(s.n)/data ->
	// L dstid[4] p0[2*4] p1[2*4] end0[4] end1[4] thick[4] srcid[4]
	// "see line in draw(2) for more details"
	// Best guess, draw a line across :
	// L  s.N
	//    Min.X (p0)
	//    Min.Y (p0)
	//    Max.X (p1)
	//    Max.Y (p1)
	//    Endsquare -> look up value (in draw(2) => Endsquare=0)
	//    Endsquare
	//    colorid
}

func newTextureImpl(s *screenImpl) *textureImpl {
	t := &textureImpl{}

	// TODO:allocate a new buffer
	return t
}
