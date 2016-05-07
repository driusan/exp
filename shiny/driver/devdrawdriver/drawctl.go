package devdrawdriver

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// A DrawCtrler is an object which holds references to
// /dev/draw/n/data and ctl, and allows you to send or
// receive messages from it.
type DrawCtrler struct {
	N    int
	ctl  io.ReadWriteCloser
	data io.ReadWriteCloser

	// the maxmum message size that can be written to
	// /dev/draw/data.
	iounitSize int
	// the next available ID to use when allocating
	// an image
	nextId uint32
}

// A DrawCtlMsg represents the data that is returned from
// opening /dev/draw/new or reading /dev/draw/n/ctl.
type DrawCtlMsg struct {
	N int

	DisplayImageId int
	ChannelFormat  string
	MysteryValue   string
	DisplaySize    image.Rectangle
	Clipping       image.Rectangle
}

// NewDrawCtrler creates a new DrawCtrler to interact with
// the /dev/draw filesystem. It returns a reference to
// a DrawCtrler, and a DrawCtlMsg representing the data
// that was returned from opening /dev/draw/new.
func NewDrawCtrler() (*DrawCtrler, *DrawCtlMsg) {
	filename := "/dev/draw/new"
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", filename, err)
		return nil, nil
	}

	// id 1 reserved for the image represented by /dev/winname, so start
	// allocating new IDs at 2.
	dc := DrawCtrler{nextId: 2}
	ctlString := dc.readCtlString(f)
	msg := parseCtlString(ctlString)
	if msg == nil {
		fmt.Fprintf(os.Stderr, "Could not parse ctl string from %s: %s\n", filename, ctlString)
		return &dc, nil
	}

	if msg.N >= 1 {
		dc.N = msg.N
		// open the data channel for the connection we just created so we can send messages to it.
		// We don't close it so that it doesn't disappear from the /dev filesystem on us.
		// It needs to be closed when the screen is cleaned up.
		dfilename := fmt.Sprintf("/dev/draw/%d/data", msg.N)
		f, err := os.OpenFile(dfilename, os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", dfilename, err)
			return &dc, msg
		}
		dc.data = f

		pid := os.Getpid() //env("pid")
		if fdInfo, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/fd", pid)); err == nil {
			fmt.Printf("PID: %d, %s\n", pid, fdInfo)
		} else {
			//fmt.Printf("Could not get info: %s\n", fdInfo)
		}
		// TODO: Read this above by parsing /proc/$pid/fd
		dc.iounitSize = 65510

		/*
			// open the ctl file, even though we don't really use it
			cfilename := fmt.Sprintf("/dev/draw/%d/ctl", msg.N)
			ctlfile, err := os.OpenFile(cfilename, os.O_RDWR, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not open %s: %s\n", dfilename, err)
				return &dc, msg
			}
			dc.ctl = ctlfile
		*/
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
		fmt.Fprintf(os.Stderr, "Error reading control string: %s\n", err)
		return ""
	}
	// there are 12 11 character wide strings in a ctl message, each followed
	// by a space. The last one may or may not have a terminating space.
	if n < 143 {
		fmt.Fprintf(os.Stderr, "Incorrect number of bytes in ctl string: %d\n", n)
		return ""
	}
	return string(val)
}

// sendMessage sends the command represented by cmd to the data channel,
// with the raw arguments in val (n.b. They need to be in little endian
// byte order and match the cmd arguments described in draw(3))
func (d DrawCtrler) sendMessage(cmd byte, val []byte) error {
	realCmd := append([]byte{cmd}, val...)
	_, err := d.data.Write(realCmd)
	if err != nil {
		panic(err)
	}
	return err
}

// Sends a message to /dev/draw/n/ctl.
// This isn't used, but might be in the future.
func (d DrawCtrler) sendCtlMessage(val []byte) error {
	_, err := d.ctl.Write(val)
	return err
}

// AllocBuffer will send a message to /dev/draw/N/data of the form:
//    b id[4] screenid[4] refresh[1] chan[4] repl[1] r[4*r] clipr[4*4] color[4]
// see draw(3) for details.
//
// For the purposes of the using this helper method, id and screenid are
// automatically generated by the DrawDriver, and chan is always an RGBA
// channel,
//
// This returns the ID that can be used to reference the allocated buffer
func (d *DrawCtrler) AllocBuffer(refresh byte, repl bool, r, clipr image.Rectangle, color color.Color) uint32 {
	msg := make([]byte, 50)
	// id is the next available ID.
	d.nextId += 1
	newId := d.nextId
	binary.LittleEndian.PutUint32(msg[0:], newId)
	// refresh can just be passed along directly.
	msg[8] = refresh
	// RGBA channel. This is the same format as image.RGBA.Pix,
	// so that we can directly upload a buffer.
	msg[9] = 8   // r8
	msg[10] = 24 // g8
	msg[11] = 40 // b8
	msg[12] = 72 // a8
	// Convert repl from bool to a byte
	if repl == true {
		msg[13] = 1
	}

	// Convert the rectangle to little endian in the appropriate
	// places
	binary.LittleEndian.PutUint32(msg[14:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[18:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(msg[22:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(msg[26:], uint32(r.Max.Y))
	binary.LittleEndian.PutUint32(msg[30:], uint32(clipr.Min.X))
	binary.LittleEndian.PutUint32(msg[34:], uint32(clipr.Min.Y))
	binary.LittleEndian.PutUint32(msg[38:], uint32(clipr.Max.X))
	binary.LittleEndian.PutUint32(msg[42:], uint32(clipr.Max.Y))
	// RGBA colour to use by default for this buffer.
	// color.RGBA() returns a uint16 (actually a uint32
	// with only the lower 16 bits set), so shift it to
	// convert it to a uint8.
	rd, g, b, a := color.RGBA()
	msg[46] = byte(a >> 8)
	msg[47] = byte(b >> 8)
	msg[48] = byte(g >> 8)
	msg[49] = byte(rd >> 8)

	d.sendMessage('b', msg)
	return newId
}

// FreeID will release the resources held by the imageID in this
// /dev/draw interface.
func (d *DrawCtrler) FreeID(id uint32) {
	// just convert to little endian and send the id to 'f'
	msg := make([]byte, 4)
	binary.LittleEndian.PutUint32(msg, id)
	d.sendMessage('f', msg)
}

// SetOp sets the compositing operation for the next draw to op
func (d *DrawCtrler) SetOp(op draw.Op) {
	// valid options according to draw(2):
	//	Clear = 0
	//	SinD  = 8
	//	DinS  = 4
	//	SoutD = 2
	//	DoutS = 1
	//	S     = SinD|SoutD (== 10)
	//	SoverD= SinD|SoutD|DoutS (==11)
	// etc.. but S and SoverD are the only valid
	// draw ops in Go
	msg := make([]byte, 1)
	switch op {
	case draw.Src:
		msg[0] = 10
	case draw.Over:
		fallthrough
	default:
		msg[0] = 11
	}
	d.sendMessage('O', msg)
}

// Draw format the parameters appropriate to send the message
//    d dstid[4] srcid[4] maskid[4] dstr[4*4] srcp[2*4] maskp[2*4]
// to /dev/draw/n/data.
// See draw(3) for details
func (d *DrawCtrler) Draw(dstid, srcid, maskid uint32, r image.Rectangle, srcp, maskp image.Point) {
	msg := make([]byte, 44)
	binary.LittleEndian.PutUint32(msg[0:], dstid)
	binary.LittleEndian.PutUint32(msg[4:], srcid)
	binary.LittleEndian.PutUint32(msg[8:], maskid)
	binary.LittleEndian.PutUint32(msg[12:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[16:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(msg[20:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(msg[24:], uint32(r.Max.Y))
	binary.LittleEndian.PutUint32(msg[28:], uint32(srcp.X))
	binary.LittleEndian.PutUint32(msg[32:], uint32(srcp.Y))
	binary.LittleEndian.PutUint32(msg[36:], uint32(maskp.X))
	binary.LittleEndian.PutUint32(msg[40:], uint32(maskp.Y))
	d.sendMessage('d', msg)
}

// ReplaceSubimage replaces the rectangle r with the pixel buffer
// defined by pixels.
//
// It sends /dev/draw/n/data the message:
//	y id[4] r[4*4] buf[x*1]
func (d *DrawCtrler) ReplaceSubimage(dstid uint32, r image.Rectangle, pixels []byte) {
	// 9p seems to be have an implicit limit of 65536 bytes that writing
	// to /dev/draw/n/data can handle. So as a hack, send one
	// line at a time if it's too big..
	rSize := r.Size()
	if (rSize.X*rSize.Y*4 + 21) < d.iounitSize {
		msg := make([]byte, 20+(rSize.X*rSize.Y*4))
		binary.LittleEndian.PutUint32(msg[0:], dstid)
		binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
		binary.LittleEndian.PutUint32(msg[8:], uint32(r.Min.Y))
		binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
		binary.LittleEndian.PutUint32(msg[16:], uint32(r.Max.Y))

		copy(msg[20:], pixels)
		d.sendMessage('y', msg)
		return
	}

	lineSize := d.iounitSize / 4 / rSize.X
	msg := make([]byte, 20+(rSize.X*lineSize*4))
	binary.LittleEndian.PutUint32(msg[0:], dstid)
	binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
	for i := r.Min.Y; i < r.Max.Y; i += lineSize {
		endline := i + lineSize
		if endline > r.Max.Y {
			endline = r.Max.Y
			msg = make([]byte, 20+(rSize.X*(endline-i)*4))
			binary.LittleEndian.PutUint32(msg[0:], dstid)
			binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
			binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))

		}
		binary.LittleEndian.PutUint32(msg[8:], uint32(i))
		binary.LittleEndian.PutUint32(msg[16:], uint32(endline))
		copy(msg[20:], pixels[i*rSize.X*4:])
		d.sendMessage('y', msg)
	}
}

// ReadSubimage returns the pixel data of the rectangle r from the
// image identified by imageID src
//
// It sends /dev/draw/n/data the message:
//	r id[4] r[4*4]
//
// and then reads the data from /dev/draw/n/data.
func (d *DrawCtrler) ReadSubimage(src uint32, r image.Rectangle) []uint8 {
	rSize := r.Size()
	msg := make([]byte, 20)
	pixels := make([]byte, (rSize.X * rSize.Y * 4))

	if (rSize.X * rSize.Y * 4) < d.iounitSize {
		binary.LittleEndian.PutUint32(msg[0:], src)
		binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
		binary.LittleEndian.PutUint32(msg[8:], uint32(r.Min.Y))
		binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
		binary.LittleEndian.PutUint32(msg[16:], uint32(r.Max.Y))

		d.sendMessage('r', msg)

		_, err := d.data.Read(pixels)
		if err != nil {
			panic(err)
		}
		return pixels
	}
	// This has the same limitation of 65536 bytes as
	// the 'y' command. Trying to read more than that will
	// return 0 bytes and an Eshortread error.
	// So, again, split it up into multiple reads and reconstruct
	// it.

	// TODO: This should calculate the maximum number of lines
	// that can fit in one message and send a rectangle of that
	// size, instead of 1 line at a time.
	binary.LittleEndian.PutUint32(msg[0:], src)
	binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
	lineSize := d.iounitSize / 4 / rSize.X

	for i := r.Min.Y; i < r.Max.Y; i += lineSize {
		endline := i + lineSize
		if endline > r.Max.Y {
			endline = r.Max.Y
		}
		binary.LittleEndian.PutUint32(msg[8:], uint32(i))
		binary.LittleEndian.PutUint32(msg[16:], uint32(endline))
		pixelsOffset := (i - r.Min.Y) * rSize.X * 4
		d.sendMessage('r', msg)
		_, err := d.data.Read(pixels[pixelsOffset:])
		if err != nil {
			panic(err)
		}
	}
	return pixels
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
