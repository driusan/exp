// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"golang.org/x/exp/shiny/driver/internal/event"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
	"image"
	"image/color"
	"image/draw"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/paint"
)

type windowImpl struct {
	s *screenImpl
	event.Queue
	winImageId windowId
	position   image.Rectangle
	resources []uint32
}

func (w *windowImpl) Release() {
	for _, id := range w.resources {
		w.s.ctl.FreeID(id)
	}
	w.s.ctl.FreeID(uint32(w.winImageId))
}

func (w *windowImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
}

func (w *windowImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {

	rect := image.Rectangle{image.ZP, dr.Size()}
	fillID := w.s.ctl.AllocBuffer(0, true, rect, rect, src)
	w.resources = append(w.resources, fillID)

	w.s.ctl.SetOp(op)
	w.s.ctl.Draw(uint32(w.winImageId), fillID, uint32(w.winImageId), dr, image.ZP, image.ZP)
}

func (w *windowImpl) Draw(src2dst f64.Aff3, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
}

func (w *windowImpl) Copy(dp image.Point, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
}

func (w *windowImpl) Scale(dr image.Rectangle, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
}

func (w *windowImpl) Publish() screen.PublishResult {
	return screen.PublishResult{false}
}

func newWindowImpl(s *screenImpl) *windowImpl {
	// now allocate a /dev/draw image to represent our window. 
	// It has the same size as the current Plan 9 image, but in it's
	// internal coordinate system the origin is 0, 0
	// default to a black background for testing.
	r := image.Rectangle{image.ZP, s.windowFrame.Size()}
	winId := s.ctl.AllocBuffer(0, false, r, r, color.RGBA{0, 0, 0, 255})

	// white background, go back to this before sending a patch, because
	// it's more plan-9-y..
//	winId := s.ctl.AllocBuffer(0, false, image.Rectangle{image.ZP, s.windowFrame.Size()}, color.RGBA{255, 255, 255, 255})
	w := &windowImpl{
		s:          s,
		winImageId: windowId(winId),
		resources: make([]uint32, 0),
	}
	w.Queue.Send(size.Event{WidthPx: r.Max.X, HeightPx: r.Max.Y})
	w.Queue.Send(paint.Event{})
	return w
}
