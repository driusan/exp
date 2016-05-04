// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"fmt"
	//	"encoding/binary"
	"golang.org/x/exp/shiny/driver/internal/event"
	"golang.org/x/exp/shiny/driver/internal/drawer"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"image"
	"image/color"
	"image/draw"
)

type windowId uint32

type windowImpl struct {
	uploadImpl
	s *screenImpl
	event.Queue
	winImageId windowId
}

func (w *windowImpl) Draw(src2dst f64.Aff3, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	return
	// There's no direct way to do an affine transformation in /dev/draw,
	// so this does the following steps:
	//
	// 1. Read the pixel data of the rectangle sr from texture.
	// 2. Transform into dst space using src2dst
	// 3. Create a new imageId of the transformed texture
	// 4. Upload the transformed data to the new ImageId
	// 3. SetOp
	// 4. Draw.
	t := src.(*textureImpl)
	pixels := w.s.ctl.ReadSubimage(uint32(t.textureId), sr)
	fmt.Printf("Data: %x\n", pixels)
//func (d *DrawCtrler) ReadSubimage(src uint32, r image.Rectangle) []uint8 {
	//panic("Done drawing")

}

func (w *windowImpl) Copy(dp image.Point, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	drawer.Copy(w, dp, src, sr, op, opts)
}

func (w *windowImpl) Scale(dr image.Rectangle, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	drawer.Scale(w, dr, src, sr, op, opts)
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

	// white background, go back to this before sending a patch, because
	// it's more plan-9-y..
	//	winId := s.ctl.AllocBuffer(0, false, image.Rectangle{image.ZP, s.windowFrame.Size()}, color.RGBA{255, 255, 255, 255})
	uploader := newUploadImpl(s, r, color.RGBA{0, 0, 0, 255})
	w := &windowImpl{
		uploadImpl: uploader,
		s:          s,
		winImageId: windowId(uploader.imageId),
	}
	w.Queue.Send(size.Event{WidthPx: r.Max.X, HeightPx: r.Max.Y})
	w.Queue.Send(paint.Event{})
	redrawWindow(s, s.windowFrame)
	return w
}
