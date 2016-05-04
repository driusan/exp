// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	//"os"
	//"image/jpeg"
	//"fmt"
	//	"encoding/binary"
	"golang.org/x/exp/shiny/driver/internal/drawer"
	"golang.org/x/exp/shiny/driver/internal/event"
	"golang.org/x/exp/shiny/screen"
	xdraw "golang.org/x/image/draw"
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
	// There's no direct way to do an affine transformation in /dev/draw,
	// so this does the following steps:
	//
	// 1. Read the pixel data of the rectangle sr from texture.
	// 2. Transform into dst space using src2dst
	// 3. Create a new imageId of the transformed texture
	// 4. Upload the transformed data to the new ImageId
	// 3. SetOp
	// 4. Draw.

	// step 1: read the subimage data
	t := src.(*textureImpl)
	pixels := w.s.ctl.ReadSubimage(uint32(t.textureId), sr)
	// convert it to an image.RGBA to make life easier.
	srcImage := image.NewRGBA(sr)
	srcImage.Pix = pixels

	// step 2: transform it into dst space
	// 2a. Calculate the size of the translated buffer by multiplying
	// the transformation through on sr.Min and sr.Max

	// helper function to do the calculations of src2dst..
	mapPoint := func(p image.Point) image.Point {
		xf, yf := float64(p.X), float64(p.Y)
		return image.Point{
			X: int(xf*src2dst[0] + yf*src2dst[1] + src2dst[2]),
			Y: int(xf*src2dst[3] + yf*src2dst[4] + src2dst[5]),
		}
	}
	// map the top left corner, and assume it's both the min and the max
	topLeft := mapPoint(sr.Min)
	min, max := topLeft, topLeft
	updateMinMax := func(p image.Point) {
		if p.X < min.X {
			min.X = p.X
		}
		if p.Y < min.Y {
			min.Y = p.Y
		}
		if p.X > max.X {
			max.X = p.X
		}
		if p.Y > max.Y {
			max.Y = p.Y
		}
	}
	// map the top right corner, and change the min or max as necessary
	p := mapPoint(image.Point{sr.Max.X, sr.Min.Y})
	updateMinMax(p)
	// bottom left
	p = mapPoint(image.Point{sr.Min.X, sr.Max.Y})
	updateMinMax(p)
	// bottom right
	p = mapPoint(image.Point{sr.Max.X, sr.Max.Y})
	updateMinMax(p)

	newRectangle := image.Rectangle{min, max}
	//fmt.Printf("New Rectangle: %s\n", newRectangle)
	// 2b. Do the transformation itself. Create a new RGBA image to
	// use temporarily to make this easier.
	transformedImage := image.NewRGBA(newRectangle)
	xdraw.NearestNeighbor.Transform(transformedImage, src2dst, srcImage, sr, xdraw.Op(op), nil)

	// 3. Create a new imageId of the transformed texture
	newOriginRectangle := image.Rectangle{image.ZP, newRectangle.Size()}
	imageId := w.s.ctl.AllocBuffer(0, false, newOriginRectangle, newOriginRectangle, color.RGBA{0, 0, 0, 0})

	// 4. Upload the transformed data to the new ImageId
	w.s.ctl.ReplaceSubimage(imageId, newOriginRectangle, transformedImage.Pix)
	// 3. SetOp
	w.s.ctl.SetOp(op)
	// 4. Draw.
	w.s.ctl.Draw(uint32(w.winImageId), imageId, imageId, newRectangle, image.ZP, image.ZP)
	// the image is already used, so we might as well free it.
	w.s.ctl.FreeID(imageId)

}

func (w *windowImpl) Copy(dp image.Point, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	drawer.Copy(w, dp, src, sr, op, opts)
}

func (w *windowImpl) Scale(dr image.Rectangle, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	drawer.Scale(w, dr, src, sr, op, opts)
}

func (w *windowImpl) Publish() screen.PublishResult {
	repositionWindow(w.s, w.s.windowFrame)
	redrawWindow(w.s, w.s.windowFrame)
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
