// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/exp/shiny/screen"
	"image"
	//"sigint.ca/plan9/draw"
	"image/draw"
	"io/ioutil"
)

type screenId uint32

type screenImpl struct {
	// the active shiny window
	w *windowImpl

	screenId screenId
	// the reference to /dev/draw/N/data to send
	// messages to
	ctl *DrawCtrler

	// the Plan 9 window that we're overlaying our shiny window
	// onto.
	windowFrame image.Rectangle

	// list of existing window image IDs that have been allocated, so we know
	// what to free at the end.
	windows []windowId
}

func (s *screenImpl) NewBuffer(size image.Point) (retBuf screen.Buffer, retErr error) {
	img := image.NewRGBA(image.Rectangle{image.ZP, size})
	return &bufferImpl{img}, nil

}

func (s *screenImpl) NewTexture(size image.Point) (screen.Texture, error) {
	return newTextureImpl(s, size), nil
}

func (s *screenImpl) NewWindow(opts *screen.NewWindowOptions) (screen.Window, error) {
	w := newWindowImpl(s)
	s.w = w
	s.windows = append(s.windows, w.winImageId)
	return w, nil
}

func (s *screenImpl) release() {
	if s == nil || s.ctl == nil {
		return
	}
	s.ctl.FreeScreen(s.screenId)
}
func newScreenImpl() (*screenImpl, error) {
	ctrl, _, err := NewDrawCtrler()
	if err != nil {
		return nil, fmt.Errorf("new controller: %v", err)
	}

	// makes image ID 0 refer to the same image as /dev/winname on this process.
	ctrl.sendMessage('n', attachWindow())

	sId, err := ctrl.AllocScreen()
	if err != nil {
		return nil, err
	}

	return &screenImpl{
		ctl:      ctrl,
		windows:  make([]windowId, 0),
		screenId: sId,
	}, nil
}

// moves the current shiny windows to be overlaid on the current plan9 window
// frame.
func repositionWindow(s *screenImpl, r image.Rectangle) {
	// reattach the window after a resize event. We always attach id 0
	// to the current window.
	s.ctl.sendMessage('f', []byte{0, 0, 0, 0})
	s.ctl.sendMessage('n', attachWindow())

	args := make([]byte, 20)
	// 0-3 = windowId
	// 4-7 = internal X. Always 0.
	// 8-11 = internal Y. Always 0.
	// 12-15 = top corner X on screen. The same as the windowFrame
	// 16-19 = top corner Y. The same as the windowFrame.
	binary.LittleEndian.PutUint32(args[12:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(args[16:], uint32(r.Min.Y))
	for _, winId := range s.windows {
		binary.LittleEndian.PutUint32(args[0:], uint32(winId))
		s.ctl.sendMessage('o', args)

		s.ctl.Reclip(uint32(winId), false, image.Rectangle{image.ZP, r.Size()})
		//s.ctl.Reclip(uint32(winId), false, r)
	}
}

// Redraw the shiny windows on top of the active Plan9 window that we're
// attached to
func redrawWindow(s *screenImpl, r image.Rectangle) {
	args := make([]byte, 44)

	// the rectangle clipping rectangle
	binary.LittleEndian.PutUint32(args[12:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(args[16:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(args[20:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(args[24:], uint32(r.Max.Y))
	// source point and mask point are both always 0.
	s.ctl.drawMu.Lock()
	defer s.ctl.drawMu.Unlock()
	for _, winId := range s.windows {
		// redraw each window id
		binary.LittleEndian.PutUint32(args[4:], uint32(winId))
		// use the window itself as a mask, so that it's opaque.
		// (or at least uses it's own alpha channel)
		binary.LittleEndian.PutUint32(args[8:], uint32(winId))
		s.ctl.setOp(draw.Src)
		s.ctl.sendMessage('d', args)
	}
	// flush the buffer
	s.ctl.sendMessage('v', nil)
}

func attachWindow() []byte {
	winname, err := ioutil.ReadFile("/dev/winname")
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 4+1+len(winname))
	buf[4] = byte(len(winname))
	copy(buf[5:], winname)
	return buf
}
