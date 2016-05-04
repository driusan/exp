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

type uploadImpl struct {
	ctl       *DrawCtrler
	imageId   uint32
	resources []uint32
}

func (u *uploadImpl) Release() {
	for _, id := range u.resources {
		u.ctl.FreeID(id)
	}
	u.ctl.FreeID(u.imageId)
}

func (u *uploadImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	img := src.RGBA()
	if img == nil {
		return
	}
	var subimage *image.RGBA = (img.SubImage(sr)).(*image.RGBA)

	dr := image.Rectangle{
		Min: dp,
		Max: dp.Add(sr.Size()),
	}
	u.ctl.ReplaceSubimage(u.imageId, dr, subimage.Pix)
}

func (u *uploadImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {

	rect := image.Rectangle{image.ZP, dr.Size()}
	fillID := u.ctl.AllocBuffer(0, true, rect, rect, src)
	u.resources = append(u.resources, fillID)

	u.ctl.SetOp(op)
	u.ctl.Draw(uint32(u.imageId), fillID, fillID, dr, image.ZP, image.ZP)
}

func newUploadImpl(s *screenImpl, size image.Rectangle, c color.Color) uploadImpl {
	// now allocate a /dev/draw image to represent our window.
	// It has the same size as the current Plan 9 image, but in it's
	// internal coordinate system the origin is 0, 0
	// default to a black background for testing.
	imageId := s.ctl.AllocBuffer(0, false, size, size, c)

	return uploadImpl{
		ctl:       s.ctl,
		imageId:   imageId,
		resources: make([]uint32, 0),
	}
}
