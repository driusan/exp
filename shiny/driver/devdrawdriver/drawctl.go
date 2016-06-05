package devdrawdriver

import (
	"bytes"
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
	"sync"
)

// A DrawCtrler is an object which holds references to
// /dev/draw/n/^(data ctl), and allows you to send or
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

	// A mutex to avoid race conditions with Draw/SetOp
	drawMu sync.Mutex
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

const NewScreen = "/dev/draw/new"

// NewDrawCtrler creates a new DrawCtrler to interact with
// the /dev/draw filesystem. It returns a reference to
// a DrawCtrler, and a DrawCtlMsg representing the data
// that was returned from opening /dev/draw/new.
func NewDrawCtrler() (*DrawCtrler, *DrawCtlMsg, error) {
	fNew, err := os.Open(NewScreen)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not open %s: %v\n", NewScreen, err)
	}
	defer fNew.Close()

	//      id 1 reserved for the image represented by /dev/winname, so
	//      start allocating new IDs at 2.
	dc := &DrawCtrler{nextId: 2}
	ctlString := dc.readCtlString(fNew)
	msg := parseCtlString(ctlString)
	if msg == nil {
		return dc, nil, fmt.Errorf("Could not parse ctl string from %s: %s\n", NewScreen, ctlString)
	}

	if msg.N < 1 {
		// huh? what now?
		return nil, nil, fmt.Errorf("draw index less than one: %d", msg.N)
	}
	dc.N = msg.N
	//      open the data channel for the connection we just created so
	//      we can send messages to it.  We don't close it so that it
	//      doesn't disappear from the /dev filesystem on us.  It needs
	//      to be closed when the screen is cleaned up.
	fn := fmt.Sprintf("/dev/draw/%d/data", msg.N)
	fData, err := os.OpenFile(fn, os.O_RDWR, 0)
	if err != nil {
		return dc, msg, fmt.Errorf("Could not open %s: %v\n", fn, err)
	}
	dc.data = fData

	// read the iounit size from the /proc filesystem.
	pid := os.Getpid()
	if fdInfo, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/fd", pid)); err == nil {
		lines := bytes.Split(fdInfo, []byte{'\n'})
		// See man proc(3) for a description of the format of /proc/$pid/fd that's
		// being parsed to find the iounit size
		// the first line is just the current wd, so don't range over it
		for _, line := range lines[1:] {
			fInfo := bytes.Fields(line)
			if len(fInfo) >= 10 && string(fInfo[9]) == fn {
				// found /dev/draw/N/data in the list of open files, so get
				// the iounit size of it.
				i, err := strconv.Atoi(string(fInfo[7]))
				if err != nil {
					return nil, nil, fmt.Errorf("Invalid iounit size. Could not convert to integer.")
				}
				dc.iounitSize = i
				break

			}

		}

		if dc.iounitSize == 0 {
			return nil, nil, fmt.Errorf("Could not parse iounit size.\n")
		}
	} else {
		return nil, nil, fmt.Errorf("Could not determine iounit size: %v\n", err)
	}
	return dc, msg, nil
}

// reads the output of /dev/draw/new or /dev/draw/n/ctl and returns
// it without doing any parsing.  It should be passed along to
// parseCtlString to create a *DrawCtlMsg
func (d DrawCtrler) readCtlString(f io.Reader) string {
	val := make([]byte, 256)
	n, err := f.Read(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading control string: %s\n", err)
		return ""
	}
	// there are 12 11 character wide strings in a ctl message, each followed
	// by a space. The last one may or may not have a terminating space, depending
	// on draw implementation, but it's irrelevant if it does.
	if err != nil || n < 143 {
		fmt.Fprintf(os.Stderr, "Incorrect number of bytes in ctl string: %d\n", n)
		return ""
	}
	return string(val[:144])
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
	//	rd, g, b, _ := color.RGBA()
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

// SetOp sets the compositing operation for the next draw to op.
// This isn't exposed, because it should only be called by Draw,
// which needs to apply a mutex.
func (d *DrawCtrler) setOp(op draw.Op) {
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
func (d *DrawCtrler) Draw(dstid, srcid, maskid uint32, r image.Rectangle, srcp, maskp image.Point, op draw.Op) {
	d.drawMu.Lock()
	defer d.drawMu.Unlock()

	d.setOp(op)

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

// Gets index and size of the largest prefix of pix[idx] which occurs
// before it in pix. If it doesn't find a prefix of at least size 3,
// it will claim it couldn't find any, and if it finds one of size 34,
// it will claim that's the largest that it found since that's the range
// that fits in a compressed image.
// It will search at most 1024 offsets back. If it doesn't find anything,
// it will return 0, 0 indicating that bytes should just be encoded directly.
func getLargestPrefix(pix []byte, idx int) (uint16, uint8) {
	var candidateIdx uint16
	var candidateSize uint8
	for i := int(idx - 34); i >= 0 && (idx-i < 128); i-- {
		if pix[i] == pix[idx] {
			if idx+34 >= len(pix) {
				break
			}
			for j, val := range pix[idx : idx+34] {
				if i+j >= len(pix) {
					break
				}
				//fmt.Printf("j: %d %x %x\n", j, val, pix[i+j])
				if val == pix[i+j] {
					//fmt.Printf("i+j: %d j: %d val:%x pix[i+j]: %x\n", i+j, j, val, pix[i+j])

					if j > int(candidateSize) {
						candidateSize = uint8(j)
						candidateIdx = uint16(i)
					}
				} else {
					break
				}
				if candidateSize == 34 {
					return candidateIdx, candidateSize
				}

			}
		}
	}
	if candidateSize > 2 {
		return candidateIdx, candidateSize
	}
	return 0, 0
}
func compress(pix []byte) []byte {
	val := make([]byte, 0)
	for i := 0; i < len(pix); {
		if idx, size := getLargestPrefix(pix, i); size > 2 {
			// "If the high-order bit is zero, the next 5 bits encode the
			//  length of a substring copied from previous pixels. Values
			//  from 0 to 31 encode lengths from 3 to 34. The bottom
			//  two bits of the first byte and the 8 bits of the next byte
			//  encode an offset backward from the current position in the
			//  pixel data at which the copy is to be found. Values from
			//  0 to 1023 encode offsets from 1 to 1024."
			//fmt.Printf("Hack at %d\n", idx)
			// hack to see if it crashes
			//	fmt.Printf("Encoding %d bytes\n", int(size))

			var encoding [2]byte

			// encode the length
			encoding[0] = (size - 3) << 2

			// encode the offset
			encodedOffset := uint16(i-int(idx)) - 1
			encoding[0] |= byte((encodedOffset & 0x0300) >> 8)
			encoding[1] = byte(encodedOffset & 0x00FF)
			/*
			   			// Convert from an absolute index into pix to a negative offset from
			   			// i, and subtract one to convert it to the encoding format.
			   fmt.Printf("i:%d idx: %d i-idx: %d uint16 i-idx: %d\n", i, idx, i-int(idx), uint16(i-int(idx)))
			   			encodedOffset := uint16(i-int(idx))-1
			   			// encode the offset
			   			encoding[0] |= byte((encodedOffset&0x0300) >> 8)
			   			encoding[1] = byte(encodedOffset&0x00FF)

			   			fmt.Printf("val from i: % x idx: % x\n", pix[i:i+int(size)], pix[idx:idx+uint16(size)])
			   			fmt.Printf("% x i:%d idx:%d size:%d EncodedOffset: %d encoding: %x\n", encoding, i, idx, size, encodedOffset, encoding)
			*/val = append(val, encoding[:]...)

			i += int(size)
			// TODO: Implement the actual compression here.
			//fmt.Printf("Should be putting %d from %d\n", idx, size)
		} else {
			// "In a code whose first byte has the high-order bit set, the rest
			//  of the byte encodes the length of  length of a  byte encoded
			// directly. Values from 0 to 127 encode lengths from 1 to 128
			// bytes. Subsequent bytes are the literal pixel data."
			//
			// If there were no matches, we just add as much data
			// as we can in order to give the next bit pixel a better
			// chance of finding something to match against.
			if left := len(pix) - i; left >= 128 {
				val = append(val, 0xFF)
				val = append(val, pix[i:i+128]...)

				i += 128
			} else {
				val = append(val, (0x80 | byte(left-1)))
				val = append(val, pix[i:i+left]...)

				i += left
			}
		}

	}
	return val
}

// Implements the compression format described in image(6) for use in
// 'Y' messages.
func (d *DrawCtrler) compressedReplaceSubimage(dstid uint32, r image.Rectangle, pixels []byte) {
	// "A compression block begins with two decimal strings of twelve bytes each. The first
	// number is one more than the y coordinate of the last row of the block. The second
	// is the number of bytes of compressed data in the block, not including the
	// two decimal strings. This number must not be larger than 6000.
	//
	// Pixels are encoding using a version of Lempel & Ziv's sliging window scheme LZ77."

	// There's 4 bytes per pixel in an RGBA, so for each iteration compress
	// rSize.X*4 = 1 line of data, check if it's over 6000, send the Y message
	// if so.

	blockYStart := 0
	rSize := r.Size()

	compressed := make([]byte, 0)
	// use rSize instead of r.Min.Y to make indexing into pixels easier.
	for i := 0; i < rSize.Y; i += 1 {

		rowStart := i * 4 * rSize.X
		linePixels := pixels[rowStart : rowStart+(rSize.X*4)]
		compressedLine := compress(linePixels)
		if len(compressed)+len(compressedLine) >= d.iounitSize || i == rSize.Y-1 {
			// construct the message for /dev/draw/data
			msg := make([]byte, 20+len(compressed))
			binary.LittleEndian.PutUint32(msg[0:], dstid)
			binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
			binary.LittleEndian.PutUint32(msg[8:], uint32(r.Min.Y+blockYStart))
			binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
			binary.LittleEndian.PutUint32(msg[16:], uint32(r.Min.Y+i))
			copy(msg[20:], compressed)
			//fmt.Printf("Sending with Y (%d, %d)-(%d,%d) -> %d not %d\n", r.Min.X, r.Min.Y+blockYStart, r.Max.X, r.Min.Y+i-1, len(compressed), (i-blockYStart)*rSize.X*4)
			d.sendMessage('Y', msg)

			// keep track of information for the next message
			blockYStart = i
			compressed = compressedLine //make([]byte, 6000)
		} else {
			compressed = append(compressed, compressedLine...)
		}

	}
}

// ReplaceSubimage replaces the rectangle r with the pixel buffer
// defined by pixels.
//
// It sends /dev/draw/n/data the message:
//	y id[4] r[4*4] buf[x*1]
func (d *DrawCtrler) ReplaceSubimage(dstid uint32, r image.Rectangle, pixels []byte) {
	// 9p limits the reads and writes to the iounit size, which is read from /proc/$pid/fd
	// at startup. So we need to split up the command into multiple 'y' commands of the
	// maximum iounit size if it doesn't fit in 1 message.
	if d.iounitSize < 65535 && len(pixels) > 256 {
		// the in-memory /dev/draw driver has an iounit size of 65535. If it's less than
		// that, it's because it's a remote implementation with some overhead somewhere.
		// In that case, use the compresssed 'Y' form instead and skip this.
		// Don't other with small images, because the overhead of the compression will
		// be worse than the gain. 256 is entirely arbitrary.
		d.compressedReplaceSubimage(dstid, r, pixels)
		return
	}
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
	// This has the same limitation of the 'y' command.
	// Trying to read more than iounit size will return 0 bytes
	// and an Eshortread error.
	// So, again, split it up into multiple reads and reconstruct
	// it.

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

func (d *DrawCtrler) Reclip(dstid uint32, repl bool, r image.Rectangle) {
	msg := make([]byte, 21)

	binary.LittleEndian.PutUint32(msg[0:], dstid)
	if repl {
		msg[4] = 1
	}
	binary.LittleEndian.PutUint32(msg[5:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[9:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(msg[13:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(msg[17:], uint32(r.Max.Y))
	d.sendMessage('c', msg)

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
