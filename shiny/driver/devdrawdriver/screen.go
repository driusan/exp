// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	//	"fmt"
	"golang.org/x/exp/shiny/screen"
	"image"
)

type screenImpl struct {
	w   *windowImpl
	ctl *DrawCtrler
	//screenId int
	dimensions *DrawCtlMsg
}

func (s *screenImpl) NewBuffer(size image.Point) (retBuf screen.Buffer, retErr error) {
	img := image.NewRGBA(image.Rectangle{image.ZP, size})
	return &bufferImpl{img}, nil

}

func (s *screenImpl) NewTexture(size image.Point) (screen.Texture, error) {
	return newTextureImpl(s), nil
}

func (s *screenImpl) NewWindow(opts *screen.NewWindowOptions) (screen.Window, error) {
	w := newWindowImpl(s)
	s.w = w
	return w, nil
}

func newScreenImpl() *screenImpl {
	//ctrl, msg := NewDrawCtrler(0)
	//fmt.Printf("%s, %s\n", ctrl, msg)
	// TODO: make sure the screen gets freed
	return &screenImpl{
		//screenId: conId,
		//ctl: ctrl,
		dimensions: nil, //msg,
	}
	//impl := &screenImpl{n: conId, screenId: 1}
	//impl.msger = f

	// create a new "Screen" by writing to /dev/draw/(n)/data ->
	// Need to figure out how to get fillid and imageid
	//    A id[4] imageid[4] fillid[4] public[1]
	// Best guess:
	// A 0x0001 0x0000 0x0000 0x01
	//return impl
}
