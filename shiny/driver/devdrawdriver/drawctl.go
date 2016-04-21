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
		//	filename = fmt.Sprintf("/dev/draw/%d/ctl", n)
		filename = "/dev/draw/new"
	}
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", filename, err)
		return nil, nil
	}
	// we actually don't use this, after initially parsing, so close it even though
	// we're holding a reference in the DrawCtrler returned

	dc := DrawCtrler{}
	ctlString := dc.readCtlString(f)
	msg := parseCtlString(ctlString)
	if msg == nil {
		fmt.Fprintf(os.Stderr, "Could not parse ctl string from %s: %s\n", filename, ctlString)
		return &dc, nil
	}
	/*
		if msg == nil {
			//TODO: read the clipping rectangle from /dev/wctl, and figure out where to get
			// the displaySize from (although do we even care?)
			msg = &DrawCtlMsg{
				N: n,
			}
		}*/

	/*
	msg := &DrawCtlMsg {
		N: 1,
		DisplayImageId: 1,
	}

	msg.N = 1*/
	if msg.N >= 1 {
		fmt.Printf("opening data")
		dfilename := fmt.Sprintf("/dev/draw/%d/data", msg.N)
		f, err := os.OpenFile(dfilename, os.O_RDWR, 0)
		 if err != nil {
			 fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", dfilename, err)
			return &dc, msg
		 }
		dc.data = f
		fmt.Printf("opening ctl\n")
		// don't defer close() this, because it'll be used by NewBuffer/Window/Texture later.
		// It needs to be closed when the screen is cleaned up.
cfilename := fmt.Sprintf("/dev/draw/%d/ctl", msg.N)
		ctlfile, err := os.OpenFile(cfilename, os.O_RDWR, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error converting keyboard input to raw mode. Could not open /dev/consctl.\n")
					return &dc, msg
			}
		dc.ctl = ctlfile
		fmt.Printf("returning")
			return &dc, msg
	}
	fmt.Printf("returning without having opened anything\n")

	return &dc, msg
}

func (d DrawCtrler) readCtlString(f io.Reader) string {
	var val []byte = make([]byte, 144)
	n, err := f.Read(val)
	if err != nil {
		return ""
	}
	if n != 144 {
		return ""
	}
	return string(val)
}

func (d DrawCtrler) sendMessage(cmd byte, val []byte) {
realCmd := append([]byte{cmd}, val...)
		 fmt.Printf("Cmd Size %d: %s / %x\n", len(realCmd), realCmd, realCmd)
		 n, err := d.data.Write(realCmd)
	if n != len(realCmd) || err != nil {
		fmt.Printf("Panicked on %c\n", cmd)
		panic(err)
	}
}
func (d DrawCtrler) sendCtlMessage(val []byte) {
	n, err := d.ctl.Write(val)
	if n != len(val) || err != nil {
		panic(err)
	}
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
