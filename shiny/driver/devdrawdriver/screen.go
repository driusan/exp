// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"encoding/binary"
	//"fmt"
	"golang.org/x/exp/shiny/screen"
	"image"
	//"sigint.ca/plan9/draw"
	"io/ioutil"
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
	ctrl, msg := NewDrawCtrler(0)
	//fmt.Printf("%s, %s\n", ctrl, msg)
	if ctrl != nil {
		//This draws a red screen in the top-left corner using low level /dev/draw/n/data messages, mostly for testing.

		// makes ID 0x0001 refer to the same image as /dev/winname on this process. I think?
		ctrl.sendMessage('n', attachscreen())

		//ctrl.sendMessage('v', []byte{})
		// create an image meaning the same thing as image.Uniform(red) for testing

		// create a new screen for us to use
		ctrl.sendMessage('A', []byte{0, 1, 0, 0, // create a screen with an arbitrary id
			0, 0, 0, 1, // backed by the current window
			0, 0, 0, 1, // filled with the same image
			1, // and make it public, because why not
		})
		/*
			// this creates a uniform red image.
			ctrl.sendMessage('b', []byte{0, 0, 0, 3, // imageid
				0, 0, 0, 0, // screenid
				0,              // refresh
				40, 24, 8, 104, // chan.. lovingly hand-crafted to be x8b8g8r8, the same as the reference by /dev/draw/new
				1,          // replicate bit. This image image is basically the same as &image.Uniform{red}
				0, 0, 0, 0, // r -> xmin
				0, 0, 0, 0, // ymin
				1, 0, 0, 0, // xmax
				1, 0, 0, 0, // ymax
				0, 0, 0, 0, // clipr -> xmin
				0, 0, 0, 0, // ymin
				0xff, 0xff, 0, 0, // xmax
				0xff, 0xff, 0, 0, // ymax
				255, 0, 0, 255, // rgba
			})
		*/

		// now allocate a window image. It's a 100x100 purple square.
		// TODO: This should be done from NewWindow, and have the right dimensions.
		ctrl.sendMessage('b', []byte{0, 0, 0, 2, // create an image with an arbitrary id
			0, 1, 0, 0, // on the screen we just created
			0,              // refresh. 0 => refbackup, 2 => refmesg
			40, 24, 8, 104, // chan.. lovingly hand-crafted to be x8b8g8r8, the same as the reference by /dev/draw/new
			0,          // replicate bit. This image is normal, this can't be set on an image that has 'o' sent to it
			0, 0, 0, 0, // r -> xmin
			0, 0, 0, 0, // ymin
			100, 0, 0, 0, // xmax
			100, 0, 0, 0, // ymax
			0, 0, 0, 0, // clipr -> xmin
			0, 0, 0, 0, // ymin
			100, 0, 0, 0, // xmax
			100, 0, 0, 0, // ymax
			255, 0, 255, 255, // rgba
		})

		/*
			ctrl.sendMessage('v', []byte{})
			ctrl.sendMessage('d', []byte{0, 0, 0, 2, // draw onto imageid we just created
				0, 0, 0, 1, // the uniform red srcid
				0, 0, 0, 0, // same mask as the window maskid? This seems to be treating it as a nil mask instead..
				0, 0, 0, 0, //dstr minx
				0, 0, 0, 0, //dstr miny
				255, 0, 0, 0, //dstr maxx
				255, 0, 0, 0, //dstr maxy
				0, 0, 0, 0, //src point x
				0, 0, 0, 0, //src point y
				0, 0, 0, 0, //mask point x
				0, 0, 0, 0, //mask point y
			})
		*/
		//redrawRedSquare(ctrl, image.Rectangle{image.Point{100, 100}, image.Point{256, 300}})
		// Flush the buffer. Not sure if this is needed, but why not..
		//ctrl.sendMessage('v', []byte{})
		ctrl.sendCtlMessage([]byte{0, 0, 0, 2}) // imageid
		ctrl.refresh([]byte{0, 0, 0, 2})
	}

	// TODO: make sure the screen gets freed, and clean up the ctl handlers upon main exiting
	s := &screenImpl{
		//screenId: conId,
		ctl:        ctrl,
		dimensions: msg,
	}
	return s
}

func repositionWindow(ctrl *DrawCtrler, r image.Rectangle) {
	// reattach the window after a resize event
	ctrl.sendMessage('f', []byte{0, 0, 0, 1})
	ctrl.sendMessage('n', attachscreen())
	/*
		ctrl.sendMessage('F', []byte{0, 1, 0, 0})
			ctrl.sendMessage('A', []byte{0, 1, 0, 0, // create a screen with an arbitrary id
				0, 0, 0, 1, // backed by the current window
				0, 0, 0, 1, // filled with the same image
				1, // and make it public, because why not
			})
	*/

	/*
		ctrl.sendMessage('d', []byte{
			0, 0, 0, 1, // draw onto the attached window
			0, 0, 0, 2, // our window - >0,0,0,2
			0, 0, 0, 2, // no mask
			0, 0, 0, 0, // 0,0 x 100,100  in dst
			0, 0, 0, 0,
			100, 0, 0, 0, // xmax
			100, 0, 0, 0, // ymax
			0, 0, 0, 0, // 0,0 source point
			0, 0, 0, 0,
			0, 0, 0, 0, // 0,0 mask point
			0, 0, 0, 0,
		})
		args := []byte{0, 0, 0, 2} // move image 2, our test window to the same location as wctl told us.
		args = append(args, lelong(uint32(r.Min.X))...)
		args = append(args, lelong(uint32(r.Min.Y))...)
		args = append(args,
			0, 0, 0, 0, // reposition the window's internal coordinates so 0,0 is the top corner of the window
			0, 0, 0, 0,
		)
		ctrl.sendMessage('o', args)
	*/
	args := []byte{0, 0, 0, 2} // move image 2, our test window to the same location as wctl told us.
	args = append(args,
		0, 0, 0, 0, // reposition the window's internal coordinates so 0,0 is the top corner of the window
		0, 0, 0, 0,
	)
	args = append(args, lelong(uint32(r.Min.X))...)
	args = append(args, lelong(uint32(r.Min.Y))...)
	ctrl.sendMessage('o', args)
	/*
		args := []byte{0, 0, 0, 1} // move image 1, the real window, to the same location
		args = append(args,
			0, 0, 0, 0, // reposition the window's internal coordinates so 0,0 is the top corner of the window
			0, 0, 0, 0,
		)
		args = append(args, lelong(uint32(r.Min.X))...)
		args = append(args, lelong(uint32(r.Min.Y))...)
		ctrl.sendMessage('o', args)
		/*

		args = []byte{1, // send to top
			 1, 0, // one window
			 0, 0, 0, 2} // our favourite window 0,0,0,2
		ctrl.sendMessage('t', args)
	*/
	ctrl.refresh([]byte{0, 0, 0, 2})

}

// Debugging method. Draw a red square on image 2. This doesn't seem to work.
func redrawImage2(ctrl *DrawCtrler, r image.Rectangle) {
	args := []byte{
		0, 0, 0, 1, // draw onto the attached window
		0, 0, 0, 2, // our window - >0,0,0,2
		0, 0, 0, 0, // no mask
	}
	args = append(args, lelong(uint32(r.Min.X))...)
	args = append(args, lelong(uint32(r.Min.Y))...)
	args = append(args, lelong(uint32(r.Max.X))...)
	args = append(args, lelong(uint32(r.Max.Y))...)
	args = append(args,
		0, 0, 0, 0, // 0,0 source point
		0, 0, 0, 0,
		0, 0, 0, 0, // 0,0 mask point
		0, 0, 0, 0,
	)
	ctrl.sendMessage('d', args)
	//ctrl.sendMessage('D',[]byte{1})
	/*
		ctrl.sendMessage('d', []byte{0, 0, 0, 2,// draw onto imageid we just created
			0, 0, 0, 3, // the uniform red srcid
			0, 0, 0, 0,
			0, 0, 0, 0, //dstr minx
			0, 0, 0, 0, //dstr miny
			100, 0, 0, 0, //dstr maxx
			100, 0, 0, 0, //dstr maxy
			0, 0, 0, 0, //src point x
			0, 0, 0, 0, //src point y
			0, 0, 0, 0, //mask point x
			0, 0, 0, 0, //mask point y
		})*/
	//ctrl.sendMessage('D',[]byte{0})
	//ctrl.sendMessage('v', []byte{})
	/*
		//ctrl.sendMessage('f', []byte{0, 0, 0, 2})

		args := []byte{0, 0, 0, 2, // create an image with an arbitrary id
			0, 1, 0, 0, // on the screen we just created
			0,              // refresh. 0 => refbackup
			40, 24, 8, 104, // chan.. lovingly hand-crafted to be x8b8g8r8, the same as the reference by /dev/draw/new
			0,          // replicate bit. This image is normal
		}
		// rectangle
		args = append(args,lelong(uint32(r.Min.X))...)
		args = append(args,lelong(uint32(r.Min.Y))...)
		args = append(args,lelong(uint32(r.Max.X))...)
		args = append(args,lelong(uint32(r.Max.Y))...)
		// clipping rectangle
		args = append(args,lelong(uint32(r.Min.X))...)
		args = append(args,lelong(uint32(r.Min.Y))...)
		args = append(args,lelong(uint32(r.Max.X))...)
		args = append(args,lelong(uint32(r.Max.Y))...)
		// default RGBA colour. Red.
		args = append(args, 255, 0, 0, 255) // rgba

		ctrl.sendMessage('b', args)

		args = []byte{0, 0, 0, 2, // draw onto imageid we just created
			0, 0, 0, 1, // the uniform red srcid
			0, 0, 0, 0, // same mask as the window maskid? This seems to be treating it as a nil mask instead..
		}
		args = append(args,lelong(uint32(r.Min.X))...)
		args = append(args,lelong(uint32(r.Min.Y))...)
		args = append(args,lelong(uint32(r.Max.X))...)
		args = append(args,lelong(uint32(r.Max.Y))...)
		args = append(args,
			0, 0, 0, 0, //src point x
			0, 0, 0, 0, //src point y
			0, 0, 0, 0, //mask point x
			0, 0, 0, 0, //mask point y
		)

		ctrl.sendMessage('d',args)

	*/
}
func attachscreen() []byte {
	winname, err := ioutil.ReadFile("/dev/winname")
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 4+1+len(winname))
	buf[3] = 1
	buf[4] = byte(len(winname))
	copy(buf[5:], winname)
	return buf
}

// Helper functions to convert to little endian.
// based on bplong/bpshort, but returns a byte buffer
// instead of populating one.
func lelong(n uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	return b
}

func leshort(n uint16) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b, n)
	return b
}
