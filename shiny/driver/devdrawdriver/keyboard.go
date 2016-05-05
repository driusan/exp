// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"bufio"
	"fmt"
	"golang.org/x/mobile/event/key"
	"os"
)

var currentModifiers key.Modifiers

// keyboardEventHandler writes rawon to /dev/consctl, and then continuously reads
// runes from /dev/cons and converts them to key.Event messages, which it passes
// along the notifier channel.
func keyboardEventHandler(notifier chan *key.Event) {
	ctl, err := os.OpenFile("/dev/consctl", os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting keyboard input to raw mode. Could not open /dev/consctl.\n")
		return
	}
	// Closing /dev/consctl will cause the keyboard to stop being in raw mode. So defer the close instead of
	// closing it right away.
	defer ctl.Close()
	rawon := []byte("rawon")
	n, err := ctl.Write(rawon)
	if err != nil || n != 5 {
		fmt.Fprintf(os.Stderr, "Error converting keyboard into raw mode. Could not write rawon..\n")
		return
	}

	cons, err := os.Open("/dev/cons")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open keyboard driver.\n")
		return

	}
	// *os.File doesn't implement ReadRune, and /dev/cons will return one rune at
	// a time in raw mode, so convert the file Reader to a bufio.Reader so that
	// it implements the ReadRune() interface.
	keyReader := bufio.NewReader(cons)
	for {
		r, _, err := keyReader.ReadRune()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading key from console.\n")
			continue
		}
		var code key.Code
		code, currentModifiers = RuneToCode(r)
		notifier <- &key.Event{
			Rune:      r,
			Code:      code,
			Modifiers: currentModifiers,
			Direction: key.DirPress,
		}

	}
}

// RuneToCode takes a unicode rune that came off of /dev/cons, and converts
// it back to the keycode (and modifiers) that would have been used to
// create that codepoint.
//
// BUG: This only supports ASCII right now, and only supports the shift
//      modifier. Adding ctrl should be just a matter of looking up
//	the unicode range for the ctrl-modified runes, and alt isn't possible
//	because Plan 9 doesn't pass that along /dev/cons (it's used at a lower
//	level to compose unicode codepoints that get passed to /dev/cons)
// TODO: Look into using /dev/kbmap instead.
func RuneToCode(r rune) (key.Code, key.Modifiers) {
	// first handle ones that can easily be calculated from the
	// ASCII ordering.
	if r >= 'a' && r <= 'z' {
		alphabetIndex := key.Code(r - 'a')
		return key.Code(alphabetIndex + key.CodeA), 0
	}
	if r >= 'A' && r <= 'Z' {
		alphabetIndex := key.Code(r - 'A')
		return key.Code(alphabetIndex + key.CodeA), key.ModShift
	}

	// then handle the rest
	switch r {
	// the key.Event keycode codes aren't sequential for the number
	// keys, so handle these specially
	case '0':
		return key.Code0, 0
	case '1':
		return key.Code1, 0
	case '2':
		return key.Code2, 0
	case '3':
		return key.Code3, 0
	case '4':
		return key.Code4, 0
	case '5':
		return key.Code5, 0
	case '6':
		return key.Code6, 0
	case '7':
		return key.Code7, 0
	case '8':
		return key.Code8, 0
	case '9':
		return key.Code9, 0
	case 27:
		return key.CodeEscape, 0
	case '\n':
		return key.CodeReturnEnter, 0
	case '\b':
		return key.CodeDeleteBackspace, 0
	case '\t':
		return key.CodeTab, 0
	case ' ':
		return key.CodeSpacebar, 0
	case '-':
		return key.CodeHyphenMinus, 0
	case '=':
		return key.CodeEqualSign, 0
	case '[':
		return key.CodeLeftSquareBracket, 0
	case ']':
		return key.CodeRightSquareBracket, 0
	case '\\':
		return key.CodeBackslash, 0
	case ';':
		return key.CodeSemicolon, 0
	case '\'':
		return key.CodeApostrophe, 0
	case '`':
		return key.CodeGraveAccent, 0
	case ',':
		return key.CodeComma, 0
	case '.':
		return key.CodeFullStop, 0
	case '/':
		return key.CodeSlash, 0
	// Aptly named unicode codepoints.
	case '\uf001':
		return key.CodeF1, 0
	case '\uf002':
		return key.CodeF2, 0
	case '\uf003':
		return key.CodeF3, 0
	case '\uf004':
		return key.CodeF4, 0
	case '\uf005':
		return key.CodeF5, 0
	case '\uf006':
		return key.CodeF6, 0
	case '\uf007':
		return key.CodeF7, 0
	case '\uf008':
		return key.CodeF8, 0
	case '\uf009':
		return key.CodeF9, 0
	case '\uf00a':
		return key.CodeF10, 0
	case '\uf00b':
		return key.CodeF11, 0
	case '\uf00c':
		return key.CodeF12, 0

	// Magic unicode characters that came from
	// pushing keys on my keyboard and seeing what
	// came out.
	case '\uf012':
		return key.CodeRightArrow, 0
	case '\uf011':
		return key.CodeLeftArrow, 0
	case '\uf00e':
		return key.CodeUpArrow, 0
	case '\uf800':
		return key.CodeDownArrow, 0

		/*
		   	TODO: Look up the unicode code point for these other keys in key.Event.
		   	      We can't go back to the keypad ones because there's no way to tell
		   	      if the rune came from the keypad or the normal keyboard.

		   	CodePause         Code = 72
		           CodeInsert        Code = 73
		           CodeHome          Code = 74
		           CodePageUp        Code = 75
		           CodeDeleteForward Code = 76
		           CodeEnd           Code = 77
		           CodePageDown      Code = 78


		*/
	default:
		fmt.Fprintf(os.Stderr, "Unknown unicode character %d %c %s, %u nsupported by /dev/draw driver.\n", r, r, r, r)
		return key.CodeUnknown, 0
	}
}
