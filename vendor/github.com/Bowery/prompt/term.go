// Copyright 2013-2015 Bowery, Inc.

package prompt

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

var (
	// ErrCTRLC is returned when CTRL+C is pressed stopping the prompt.
	ErrCTRLC = errors.New("Interrupted (CTRL+C)")
	// ErrEOF is returned when CTRL+D is pressed stopping the prompt.
	ErrEOF = errors.New("EOF (CTRL+D)")
)

// Possible events that may occur when reading from input.
const (
	evChar = iota
	evSkip
	evReturn
	evEOF
	evCtrlC
	evBack
	evClear
	evHome
	evEnd
	evUp
	evDown
	evRight
	evLeft
	evDel
)

// IsNotTerminal checks if an error is related to the input not being a terminal.
func IsNotTerminal(err error) bool {
	return isNotTerminal(err)
}

// TerminalSize retrieves the columns/rows for the terminal connected to out.
func TerminalSize(out *os.File) (int, int, error) {
	return terminalSize(out)
}

// Terminal contains the state for raw terminal input.
type Terminal struct {
	In           *os.File
	Out          *os.File
	History      []string
	histIdx      int
	simpleReader *bufio.Reader
	t            *terminal
}

// NewTerminal creates a terminal and sets it to raw input mode.
func NewTerminal() (*Terminal, error) {
	in := os.Stdin

	term, err := newTerminal(in)
	if err != nil {
		return nil, err
	}

	return &Terminal{
		In:      in,
		Out:     os.Stdout,
		History: make([]string, 0, 10),
		histIdx: -1,
		t:       term,
	}, nil
}

// Basic gets input and if required tests to ensure input was given.
func (term *Terminal) Basic(prefix string, required bool) (string, error) {
	return term.Custom(prefix, func(input string) (string, bool) {
		if required && input == "" {
			return "", false
		}

		return input, true
	})
}

// BasicDefault gets input and if empty uses the given default.
func (term *Terminal) BasicDefault(prefix, def string) (string, error) {
	return term.Custom(prefix+"(Default: "+def+")", func(input string) (string, bool) {
		if input == "" {
			input = def
		}

		return input, true
	})
}

// Ask gets input and checks if it's truthy or not, and returns that
// in a boolean fashion.
func (term *Terminal) Ask(question string) (bool, error) {
	input, err := term.Custom(question+"?(y/n)", func(input string) (string, bool) {
		if input == "" {
			return "", false
		}
		input = strings.ToLower(input)

		if input == "y" || input == "yes" {
			return "yes", true
		}

		return "", true
	})

	var ok bool
	if input != "" {
		ok = true
	}

	return ok, err
}

// Custom gets input and calls the given test function with the input to
// check if the input is valid, a true return will return the string.
func (term *Terminal) Custom(prefix string, test func(string) (string, bool)) (string, error) {
	var err error
	var input string
	var ok bool

	for !ok {
		input, err = term.GetPrompt(prefix)
		if err != nil && err != io.EOF {
			return "", err
		}

		input, ok = test(input)
	}

	return input, nil
}

// Password retrieves a password from stdin without echoing it.
func (term *Terminal) Password(prefix string) (string, error) {
	var err error
	var input string

	for input == "" {
		input, err = term.GetPassword(prefix)
		if err != nil && err != io.EOF {
			return "", err
		}
	}

	return input, nil
}

// GetPrompt gets a line with the prefix and echos input.
func (term *Terminal) GetPrompt(prefix string) (string, error) {
	if !term.t.supportsEditing {
		return term.simplePrompt(prefix)
	}

	buf := NewBuffer(prefix, term.Out, true)
	return term.prompt(buf, NewAnsiReader(term.In))
}

// GetPassword gets a line with the prefix and doesn't echo input.
func (term *Terminal) GetPassword(prefix string) (string, error) {
	if !term.t.supportsEditing {
		return term.simplePrompt(prefix)
	}

	buf := NewBuffer(prefix, term.Out, false)
	return term.password(buf, NewAnsiReader(term.In))
}

func (term *Terminal) Close() error {
	return term.t.Close()
}

// simplePrompt is a fallback prompt without line editing support.
func (term *Terminal) simplePrompt(prefix string) (string, error) {
	if term.simpleReader == nil {
		term.simpleReader = bufio.NewReader(term.In)
	}

	_, err := term.Out.Write([]byte(prefix))
	if err != nil {
		return "", err
	}

	line, err := term.simpleReader.ReadString('\n')
	line = strings.TrimRight(line, "\r\n ")
	line = strings.TrimLeft(line, " ")

	return line, err
}

// setup initializes a prompt.
func (term *Terminal) setup(buf *Buffer, in io.Reader) (*bufio.Reader, error) {
	cols, _, err := TerminalSize(buf.Out)
	if err != nil {
		return nil, err
	}

	buf.Cols = cols
	input := bufio.NewReader(in)

	err = buf.Refresh()
	if err != nil {
		return nil, err
	}

	return input, nil
}

// read reads a rune and parses ANSI escape sequences found
func (term *Terminal) read(in *bufio.Reader) (int, rune, error) {
	char, _, err := in.ReadRune()
	if err != nil {
		return 0, 0, err
	}

	switch char {
	default:
		// Standard chars.
		return evChar, char, nil
	case tabKey, ctrlA, ctrlB, ctrlE, ctrlF, ctrlG, ctrlH, ctrlJ, ctrlK, ctrlN,
		ctrlO, ctrlP, ctrlQ, ctrlR, ctrlS, ctrlT, ctrlU, ctrlV, ctrlW, ctrlX,
		ctrlY, ctrlZ:
		// Skip.
		return evSkip, char, nil
	case returnKey:
		// End of line.
		return evReturn, char, nil
	case ctrlD:
		// End of file.
		return evEOF, char, nil
	case ctrlC:
		// End of line, interrupted.
		return evCtrlC, char, nil
	case backKey:
		// Backspace.
		return evBack, char, nil
	case ctrlL:
		// Clear screen.
		return evClear, char, nil
	case escKey:
		// Functions like arrows, home, etc.
		esc := make([]byte, 2)
		_, err = in.Read(esc)
		if err != nil {
			return -1, char, err
		}

		// Home, end.
		if esc[0] == 'O' {
			switch esc[1] {
			case 'H':
				// Home.
				return evHome, char, nil
			case 'F':
				// End.
				return evEnd, char, nil
			}

			return evSkip, char, nil
		}

		// Arrows, delete, pgup, pgdown, insert.
		if esc[0] == '[' {
			switch esc[1] {
			case 'A':
				// Up.
				return evUp, char, nil
			case 'B':
				// Down.
				return evDown, char, nil
			case 'C':
				// Right.
				return evRight, char, nil
			case 'D':
				// Left.
				return evLeft, char, nil
			}

			// Delete, pgup, pgdown, insert.
			if esc[1] > '0' && esc[1] < '7' {
				extEsc := make([]byte, 3)
				_, err = in.Read(extEsc)
				if err != nil {
					return -1, char, err
				}

				if extEsc[0] == '~' {
					switch esc[1] {
					case '2', '5', '6':
						// Insert, pgup, pgdown.
						return evSkip, char, err
					case '3':
						// Delete.
						return evDel, char, err
					}
				}
			}
		}
	}

	return evSkip, char, nil
}

// prompt reads from in and parses ANSI escapes writing to buf.
func (term *Terminal) prompt(buf *Buffer, in io.Reader) (string, error) {
	input, err := term.setup(buf, in)
	if err != nil {
		return "", err
	}
	term.History = append(term.History, "")
	term.histIdx = len(term.History) - 1
	curHistIdx := term.histIdx

	for {
		typ, char, err := term.read(input)
		if err != nil {
			return buf.String(), err
		}

		switch typ {
		case evChar:
			err = buf.Insert(char)
			if err != nil {
				return buf.String(), err
			}

			term.History[curHistIdx] = buf.String()
		case evSkip:
			continue
		case evReturn:
			err = buf.EndLine()
			return buf.String(), err
		case evEOF:
			err = buf.EndLine()
			if err == nil {
				err = ErrEOF
			}

			return buf.String(), err
		case evCtrlC:
			err = buf.EndLine()
			if err == nil {
				err = ErrCTRLC
			}

			return buf.String(), err
		case evBack:
			err = buf.DelLeft()
			if err != nil {
				return buf.String(), err
			}

			term.History[curHistIdx] = buf.String()
		case evClear:
			err = buf.ClsScreen()
			if err != nil {
				return buf.String(), err
			}
		case evHome:
			err = buf.Start()
			if err != nil {
				return buf.String(), err
			}
		case evEnd:
			err = buf.End()
			if err != nil {
				return buf.String(), err
			}
		case evUp:
			idx := term.histIdx
			if term.histIdx > 0 {
				idx--
			}

			err = buf.Set([]rune(term.History[idx])...)
			if err != nil {
				return buf.String(), err
			}

			term.histIdx = idx
		case evDown:
			idx := term.histIdx
			if term.histIdx < len(term.History)-1 {
				idx++
			}

			err = buf.Set([]rune(term.History[idx])...)
			if err != nil {
				return buf.String(), err
			}

			term.histIdx = idx
		case evRight:
			err = buf.Right()
			if err != nil {
				return buf.String(), err
			}
		case evLeft:
			err = buf.Left()
			if err != nil {
				return buf.String(), err
			}
		case evDel:
			err = buf.Del()
			if err != nil {
				return buf.String(), err
			}

			term.History[curHistIdx] = buf.String()
		}
	}
}

// password reads from in and parses restricted ANSI escapes writing to buf.
func (term *Terminal) password(buf *Buffer, in io.Reader) (string, error) {
	input, err := term.setup(buf, in)
	if err != nil {
		return "", err
	}

	for {
		typ, char, err := term.read(input)
		if err != nil {
			return buf.String(), err
		}

		switch typ {
		case evChar:
			err = buf.Insert(char)
			if err != nil {
				return buf.String(), err
			}
		case evSkip, evHome, evEnd, evUp, evDown, evRight, evLeft, evDel:
			continue
		case evReturn:
			err = buf.EndLine()
			return buf.String(), err
		case evEOF:
			err = buf.EndLine()
			if err == nil {
				err = ErrEOF
			}

			return buf.String(), err
		case evCtrlC:
			err = buf.EndLine()
			if err == nil {
				err = ErrCTRLC
			}

			return buf.String(), err
		case evBack:
			err = buf.DelLeft()
			if err != nil {
				return buf.String(), err
			}
		case evClear:
			err = buf.ClsScreen()
			if err != nil {
				return buf.String(), err
			}
		}
	}
}
