// Copyright 2013-2015 Bowery, Inc.

// Package prompt implements a cross platform line-editing prompt. It also
// provides routines to use ANSI escape sequences across platforms for
// terminal connected io.Readers/io.Writers.
//
// If os.Stdin isn't connected to a terminal or (on Unix)if the terminal
// doesn't support the ANSI escape sequences needed a fallback prompt is
// provided that doesn't do line-editing. Unix terminals that are not supported
// will have the TERM environment variable set to either "dumb" or "cons25".
//
// The keyboard shortcuts are similar to those found in the Readline library:
//
//   - Enter / CTRL+D
//     - End the line.
//   - CTRL+C
//     - End the line, return error `ErrCTRLC`.
//   - Backspace
//     - Remove the character to the left.
//   - CTRL+L
//     - Clear the screen(keeping the current lines content).
//   - Home / End
//     - Jump to the beginning/end of the line.
//   - Up arrow / Down arrow
//     - Go back and forward in the history.
//   - Left arrow / Right arrow
//     - Move left/right one character.
//   - Delete
//     - Remove the character to the right.
package prompt

// Basic is a wrapper around Terminal.Basic.
func Basic(prefix string, required bool) (string, error) {
	term, err := NewTerminal()
	if err != nil {
		return "", err
	}
	defer term.Close()

	return term.Basic(prefix, required)
}

// BasicDefault is a wrapper around Terminal.BasicDefault.
func BasicDefault(prefix, def string) (string, error) {
	term, err := NewTerminal()
	if err != nil {
		return "", err
	}
	defer term.Close()

	return term.BasicDefault(prefix, def)
}

// Ask is a wrapper around Terminal.Ask.
func Ask(question string) (bool, error) {
	term, err := NewTerminal()
	if err != nil {
		return false, err
	}
	defer term.Close()

	return term.Ask(question)
}

// Custom is a wrapper around Terminal.Custom.
func Custom(prefix string, test func(string) (string, bool)) (string, error) {
	term, err := NewTerminal()
	if err != nil {
		return "", err
	}
	defer term.Close()

	return term.Custom(prefix, test)
}

// Password is a wrapper around Terminal.Password.
func Password(prefix string) (string, error) {
	term, err := NewTerminal()
	if err != nil {
		return "", err
	}
	defer term.Close()

	return term.Password(prefix)
}
