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
	N    int
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
	filename := "/dev/draw/new"
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", filename, err)
		return nil, nil
	}

	dc := DrawCtrler{}
	ctlString := dc.readCtlString(f)
	msg := parseCtlString(ctlString)
	if msg == nil {
		fmt.Fprintf(os.Stderr, "Could not parse ctl string from %s: %s\n", filename, ctlString)
		return &dc, nil
	}
	//TODO: read the clipping rectangle from /dev/wctl, (maybe have another goroutine monitoring it for resize
	// events?)

	if msg.N >= 1 {
		dc.N = msg.N
		// open the data channel for the connection we just created so we can send messages to it.
		//
		dfilename := fmt.Sprintf("/dev/draw/%d/data", msg.N)
		f, err := os.OpenFile(dfilename, os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", dfilename, err)
			return &dc, msg
		}
		dc.data = f
		// We don't close it so that it doesn't disappear from the /dev filesystem on us.
		// It needs to be closed when the screen is cleaned up.

		// open the ctl file, even though we don't really use it
		cfilename := fmt.Sprintf("/dev/draw/%d/ctl", msg.N)
		ctlfile, err := os.OpenFile(cfilename, os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", dfilename, err)
			return &dc, msg
		}
		dc.ctl = ctlfile
	}
	return &dc, msg
}

// reads the output of /dev/draw/new or /dev/draw/n/ctl and returns it without
// doing any parsing. It should be passed along to parseCtlString to create
// a *DrawCtlMsg
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

// sendMessage sends the command represented by cmd to the data channel,
// with the raw arguments in val (n.b. They need to be in little endian
// byte order and match the cmd arguments described in draw(3)
// TODO: Write better wrappers around this that handle endian conversions
//       and common arguments.
func (d DrawCtrler) sendMessage(cmd byte, val []byte) {
	realCmd := append([]byte{cmd}, val...)
	n, err := d.data.Write(realCmd)
	if n != len(realCmd) || err != nil {
		// TODO: Use real error handling.
		panic(err)
	}
}

// Sends a message to /dev/draw/n/ctl.
// This isn't used, but might be in the future.
func (d DrawCtrler) sendCtlMessage(val []byte) {
	n, err := d.ctl.Write(val)
	if n != len(val) || err != nil {
		// TODO: Use real error handling.
		panic(err)
	}
}

func (d DrawCtrler) refresh(val []byte) {
	d.sendCtlMessage(val)
	filename := fmt.Sprintf("/dev/draw/%d/refresh", d.N)
	f, _ := os.Open(filename)
	defer f.Close()
}

// parseCtlString parses the output of the format returned by /dev/draw/new.
// It can also be used to parse a /dev/draw/n/ctl output, but isn't currently.
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

// helper function for parseCtlstring that returns a single value instead of a multi-value
// so that it can be used inline..
func strToInt(s string) int {
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return i
}
