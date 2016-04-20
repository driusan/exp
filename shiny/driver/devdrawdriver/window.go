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
)

type windowImpl struct {
	s *screenImpl
	event.Queue
	winImageId int
}

func (w *windowImpl) Release() {
	// write to /dev/draw/(s.n)/data
	// f winImageId[4]

}

func (w *windowImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
}

func (w *windowImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {
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
	return &windowImpl{
		s: s,
	}
}
