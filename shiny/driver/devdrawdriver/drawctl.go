package devdrawdriver

import (
	"fmt"
	"image"
	"io"
	"os"
	"strconv"
	"strings"
)

type DrawCtrler struct {
	ctl  io.ReadWriteCloser
	data io.ReadWriteCloser
}

type DrawCtlMsg struct {
	N int

	DisplayImageId int
	ChannelFormat  string
	MysteryValue   string
	DisplaySize    image.Rectangle
	Clipping       image.Rectangle
}

func NewDrawCtrler(n int) (*DrawCtrler, *DrawCtlMsg) {
	var filename string
	if n == 0 {
		filename = "/dev/draw/new"
	} else {
		filename = fmt.Sprintf("/dev/draw/%d/ctl", n)
	}
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", filename, err)
		return nil, nil
	}
	dc := DrawCtrler{}
	dc.ctl = f
	// we actually don't use this, after initially parsing, so close it even though
	// we're holding a reference in the DrawCtrler returned
	defer f.Close()

	ctlString := dc.readCtlString()
	msg := parseCtlString(ctlString)
	return &dc, msg
}

func (d DrawCtrler) readCtlString() string {
	var val []byte = make([]byte, 144)
	n, err := d.ctl.Read(val)
	if err != nil {
		return ""
	}
	if n != 144 {
		return ""
	}
	return string(val)
}

func parseCtlString(drawString string) *DrawCtlMsg {
	pieces := strings.Fields(drawString)
	if len(pieces) != 12 {
		fmt.Fprintf(os.Stderr, "Invalid /dev/draw ctl string: %s\n", drawString)
		return nil
	}
	return &DrawCtlMsg{
		N:              strToInt(pieces[0]),
		DisplayImageId: strToInt(pieces[1]),
		ChannelFormat:  pieces[2],
		// the man page says there are 12 strings returned by /dev/draw/new,
		// and in fact there are, but I only count 11 described in the man page
		// pieces[3] seems to be the location of the mystery value.
		// It seems to be "0" when I just do a cat /dev/draw/new
		MysteryValue: pieces[3],
		DisplaySize: image.Rectangle{
			Min: image.Point{strToInt(pieces[4]), strToInt(pieces[5])},
			Max: image.Point{strToInt(pieces[6]), strToInt(pieces[7])},
		},
		Clipping: image.Rectangle{
			Min: image.Point{strToInt(pieces[8]), strToInt(pieces[9])},
			Max: image.Point{strToInt(pieces[10]), strToInt(pieces[11])},
		},
	}
}
func strToInt(s string) int {
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return i
}
