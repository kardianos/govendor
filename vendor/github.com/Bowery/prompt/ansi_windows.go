// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"bytes"
	"os"
	"unicode/utf8"
	"unsafe"
)

// keyEventType is the key event type for an input record.
const keyEventType = 0x0001

var (
	readConsoleInput = kernel.NewProc("ReadConsoleInputW")
)

// inputRecord describes a input event from a console.
type inputRecord struct {
	eventType uint16
	// Magic to get around the union C type, cast
	// event to the type using unsafe.Pointer.
	_     [2]byte
	event [16]byte
}

// keyEventRecord describes a keyboard event.
type keyEventRecord struct {
	keyDown         int32
	repeatCount     uint16
	virtualKeyCode  uint16
	virtualScanCode uint16
	char            uint16
	controlKeyState uint32
}

// AnsiReader is an io.Reader that reads from a given file and converts Windows
// key codes to their equivalent ANSI escape codes.
type AnsiReader struct {
	fd  uintptr
	buf []rune
}

// NewAnsiReader creates a AnsiReader from the given input file.
func NewAnsiReader(in *os.File) *AnsiReader {
	return &AnsiReader{fd: in.Fd()}
}

// Read reads data from the input converting to ANSI escape codes that can be
// read over multiple Reads.
func (ar *AnsiReader) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	if len(ar.buf) == 0 {
		var runes []rune
		var read uint32
		rec := new(inputRecord)

		for runes == nil {
			ret, _, err := readConsoleInput.Call(ar.fd, uintptr(unsafe.Pointer(rec)),
				1, uintptr(unsafe.Pointer(&read)))
			if ret == 0 {
				return 0, err
			}

			if rec.eventType != keyEventType {
				continue
			}

			ke := (*keyEventRecord)(unsafe.Pointer(&rec.event))
			if ke.keyDown == 0 {
				continue
			}

			shift := false
			if ke.controlKeyState&shiftKey != 0 {
				shift = true
			}

			ctrl := false
			if ke.controlKeyState&leftCtrlKey != 0 || ke.controlKeyState&rightCtrlKey != 0 {
				ctrl = true
			}

			alt := false
			if ke.controlKeyState&leftAltKey != 0 || ke.controlKeyState&rightAltKey != 0 {
				alt = true
			}

			// Backspace, Return, Space.
			if ke.char == ctrlH || ke.char == returnKey || ke.char == spaceKey {
				code := string(returnKey)
				if ke.char == ctrlH {
					code = string(backKey)
				} else if ke.char == spaceKey {
					code = string(spaceKey)
				}

				if alt {
					code = string(escKey) + code
				}

				runes = []rune(code)
				break
			}

			// Generate runes for the chars and key codes.
			if ke.char > 0 {
				runes = []rune{rune(ke.char)}
			} else {
				code := string(escKey)

				switch ke.virtualKeyCode {
				case f1Key:
					if ctrl {
						continue
					}

					code += ar.shortFunction("P", shift, ctrl, alt)
				case f2Key:
					code += ar.shortFunction("Q", shift, ctrl, alt)
				case f3Key:
					code += ar.shortFunction("R", shift, ctrl, alt)
				case f4Key:
					code += ar.shortFunction("S", shift, ctrl, alt)
				case f5Key:
					code += ar.longFunction("15", shift, ctrl, alt)
				case f6Key:
					code += ar.longFunction("17", shift, ctrl, alt)
				case f7Key:
					code += ar.longFunction("18", shift, ctrl, alt)
				case f8Key:
					code += ar.longFunction("19", shift, ctrl, alt)
				case f9Key:
					code += ar.longFunction("20", shift, ctrl, alt)
				case f10Key:
					code += ar.longFunction("21", shift, ctrl, alt)
				case f11Key:
					code += ar.longFunction("23", shift, ctrl, alt)
				case f12Key:
					code += ar.longFunction("24", shift, ctrl, alt)
				case insertKey:
					if shift || ctrl {
						continue
					}

					code += ar.longFunction("2", shift, ctrl, alt)
				case deleteKey:
					code += ar.longFunction("3", shift, ctrl, alt)
				case homeKey:
					code += "OH"
				case endKey:
					code += "OF"
				case pgupKey:
					if shift {
						continue
					}

					code += ar.longFunction("5", shift, ctrl, alt)
				case pgdownKey:
					if shift {
						continue
					}

					code += ar.longFunction("6", shift, ctrl, alt)
				case upKey:
					code += ar.arrow("A", shift, ctrl, alt)
				case downKey:
					code += ar.arrow("B", shift, ctrl, alt)
				case leftKey:
					code += ar.arrow("D", shift, ctrl, alt)
				case rightKey:
					code += ar.arrow("C", shift, ctrl, alt)
				default:
					continue
				}

				runes = []rune(code)
			}
		}

		ar.buf = runes
	}

	// Get items from the buffer.
	var n int
	for i, r := range ar.buf {
		if utf8.RuneLen(r) > len(b) {
			ar.buf = ar.buf[i:]
			return n, nil
		}

		nr := utf8.EncodeRune(b, r)
		b = b[nr:]
		n += nr
	}

	ar.buf = nil
	return n, nil
}

// shortFunction creates a short function code.
func (ar *AnsiReader) shortFunction(ident string, shift, ctrl, alt bool) string {
	code := "O"

	if shift {
		code += "1;2"
	} else if ctrl {
		code += "1;5"
	} else if alt {
		code += "1;3"
	}

	return code + ident
}

// longFunction creates a long function code.
func (ar *AnsiReader) longFunction(ident string, shift, ctrl, alt bool) string {
	code := "["
	code += ident

	if shift {
		code += ";2"
	} else if ctrl {
		code += ";5"
	} else if alt {
		code += ";3"
	}

	return code + "~"
}

// arrow creates an arrow code.
func (ar *AnsiReader) arrow(ident string, shift, ctrl, alt bool) string {
	code := "["

	if shift {
		code += "1;2"
	} else if ctrl {
		code += "1;5"
	} else if alt {
		code += "1;3"
	}

	return code + ident
}

// AnsiWriter is an io.Writer that writes to a given file and converts ANSI
// escape codes to their equivalent Windows functionality.
type AnsiWriter struct {
	file *os.File
	buf  []byte
}

// NewAnsiWriter creates a AnsiWriter from the given output.
func NewAnsiWriter(out *os.File) *AnsiWriter {
	return &AnsiWriter{file: out}
}

// Write writes the buffer filtering out ANSI escape codes and converting to
// the Windows functionality needed. ANSI escape codes may be found over multiple
// Writes.
func (aw *AnsiWriter) Write(b []byte) (int, error) {
	needsProcessing := bytes.Contains(b, []byte(string(escKey)))
	if len(aw.buf) > 0 {
		needsProcessing = true
	}

	if !needsProcessing {
		return aw.file.Write(b)
	}
	var p []byte

	for _, char := range b {
		// Found the beginning of an escape.
		if char == escKey {
			aw.buf = append(aw.buf, char)
			continue
		}

		// Funtion identifiers.
		if len(aw.buf) == 1 && (char == '_' || char == 'P' || char == '[' ||
			char == ']' || char == '^' || char == ' ' || char == '#' ||
			char == '%' || char == '(' || char == ')' || char == '*' ||
			char == '+') {
			aw.buf = append(aw.buf, char)
			continue
		}

		// Cursor functions.
		if len(aw.buf) == 1 && (char == '7' || char == '8') {
			// Add another char before because finish skips 2 items.
			aw.buf = append(aw.buf, '_', char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Keyboard functions.
		if len(aw.buf) == 1 && (char == '=' || char == '>') {
			aw.buf = append(aw.buf, char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Bottom left function.
		if len(aw.buf) == 1 && char == 'F' {
			// Add extra char for finish.
			aw.buf = append(aw.buf, '_', char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Reset function.
		if len(aw.buf) == 1 && char == 'c' {
			// Add extra char for finish.
			aw.buf = append(aw.buf, '_', char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Space functions.
		if len(aw.buf) >= 2 && aw.buf[1] == ' ' && (char == 'F' || char == 'G' ||
			char == 'L' || char == 'M' || char == 'N') {
			aw.buf = append(aw.buf, char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Number functions.
		if len(aw.buf) >= 2 && aw.buf[1] == '#' && (char >= '3' && char <= '6') ||
			char == '8' {
			aw.buf = append(aw.buf, char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Percentage functions.
		if len(aw.buf) >= 2 && aw.buf[1] == '%' && (char == '@' || char == 'G') {
			aw.buf = append(aw.buf, char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// Character set functions.
		if len(aw.buf) >= 2 && (aw.buf[1] == '(' || aw.buf[1] == ')' ||
			aw.buf[1] == '*' || aw.buf[1] == '+') && (char == '0' ||
			(char >= '4' && char <= '7') || char == '=' || (char >= 'A' &&
			char <= 'C') || char == 'E' || char == 'H' || char == 'K' ||
			char == 'Q' || char == 'R' || char == 'Y') {
			aw.buf = append(aw.buf, char)

			err := aw.finish(nil)
			if err != nil {
				return 0, err
			}

			continue
		}

		// APC functions.
		if len(aw.buf) >= 2 && aw.buf[1] == '_' {
			aw.buf = append(aw.buf, char)

			// End of APC.
			if char == '\\' && aw.buf[len(aw.buf)-1] == escKey {
				err := aw.finish(nil)
				if err != nil {
					return 0, err
				}
			}

			continue
		}

		// DC functions.
		if len(aw.buf) >= 2 && aw.buf[1] == 'P' {
			aw.buf = append(aw.buf, char)

			// End of DC.
			if char == '\\' && aw.buf[len(aw.buf)-1] == escKey {
				err := aw.finish(nil)
				if err != nil {
					return 0, err
				}
			}

			continue
		}

		// CSI functions.
		if len(aw.buf) >= 2 && aw.buf[1] == '[' {
			aw.buf = append(aw.buf, char)

			// End of CSI.
			if char == '@' || (char >= 'A' && char <= 'M') || char == 'P' ||
				char == 'S' || char == 'T' || char == 'X' || char == 'Z' ||
				char == '`' || (char >= 'b' && char <= 'd') || (char >= 'f' &&
				char <= 'i') || (char >= 'l' && char <= 'n') || (char >= 'p' &&
				char <= 't') || char == 'w' || char == 'x' || char == 'z' ||
				char == '{' || char == '|' {
				err := aw.finish(nil)
				if err != nil {
					return 0, err
				}
			}

			continue
		}

		// OSC functions.
		if len(aw.buf) >= 2 && aw.buf[1] == ']' {
			aw.buf = append(aw.buf, char)

			// Capture incomplete code.
			if len(aw.buf) == 4 && aw.buf[2] == '0' && char == ';' {
				err := aw.finish(nil)
				if err != nil {
					return 0, err
				}

				continue
			}

			// End of OSC.
			if (char == '\\' && aw.buf[len(aw.buf)-1] == escKey) || char == ctrlG {
				err := aw.finish(nil)
				if err != nil {
					return 0, err
				}
			}

			continue
		}

		// PM functions.
		if len(aw.buf) >= 2 && aw.buf[1] == '^' {
			aw.buf = append(aw.buf, char)

			// End of PM.
			if char == '\\' && aw.buf[len(aw.buf)-1] == escKey {
				err := aw.finish(nil)
				if err != nil {
					return 0, err
				}
			}

			continue
		}

		// Normal character, resets escape buffer.
		if len(aw.buf) > 0 {
			aw.buf = nil
		}
		p = append(p, char)
	}

	_, err := aw.file.Write(p)
	return len(b), err
}

// finish finishes an ANSI escape code and calls the parsing function. Afterwards
// the escape buffer is emptied.
func (aw *AnsiWriter) finish(parse func([]byte) error) error {
	var err error

	if parse != nil {
		err = parse(aw.buf[2:])
	}

	aw.buf = nil
	return err
}
