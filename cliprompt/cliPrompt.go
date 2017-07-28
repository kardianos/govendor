// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cliprompt uses the CLI to prompt for user feedback.
package cliprompt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kardianos/govendor/prompt"

	cp "github.com/Bowery/prompt"
)

type Prompt struct{}

// Ask the user a question based on the CLI.
// TODO (DT): Currently can't handle fetching empty responses do to cancel method.
func (p *Prompt) Ask(q *prompt.Question) (prompt.Response, error) {
	term, err := cp.NewTerminal()
	if err != nil {
		return prompt.RespCancel, err
	}

	if len(q.Error) > 0 {
		fmt.Fprintf(term.Out, "%s\n\n", q.Error)
	}

	switch q.Type {
	default:
		panic("Unknown question type")
	case prompt.TypeSelectMultiple:
		return prompt.RespCancel, fmt.Errorf("Selecting multiple isn't currently supported")
	case prompt.TypeSelectOne:
		return getSingle(term, q)
	}
}

func getSingle(term *cp.Terminal, q *prompt.Question) (prompt.Response, error) {
	if len(q.Options) == 1 && q.Options[0].Other() {
		opt := &q.Options[0]
		opt.Chosen = true
		return setOther(term, q, opt)
	}

	chosen := q.AnswerSingle(false)
	if chosen == nil {
		return setOption(term, q)
	}
	resp, err := setOther(term, q, chosen)
	if err != nil {
		return prompt.RespCancel, err
	}
	if resp == prompt.RespCancel {
		chosen.Chosen = false
		return setOption(term, q)
	}
	return resp, nil
}

func setOther(term *cp.Terminal, q *prompt.Question, opt *prompt.Option) (prompt.Response, error) {
	var blankCount = 0
	var internalMessage = ""
	for {
		// Write out messages
		if len(internalMessage) > 0 {
			fmt.Fprintf(term.Out, "%s\n\n", internalMessage)
		}
		if len(q.Prompt) > 0 {
			fmt.Fprintf(term.Out, "%s\n", q.Prompt)
		}
		if len(opt.Validation()) > 0 {
			fmt.Fprintf(term.Out, "  ** %s\n", opt.Validation())
		}
		// Reset message.
		internalMessage = ""
		ln, err := term.Basic(" > ", false)
		if err != nil {
			return prompt.RespCancel, err
		}
		if len(ln) == 0 && blankCount > 0 {
			return prompt.RespCancel, nil
		}
		if len(ln) == 0 {
			internalMessage = "Press enter again to cancel"
			blankCount++
			continue
		}
		blankCount = 0
		opt.Value = strings.TrimSpace(ln)
		return prompt.RespAnswer, nil
	}
}

func setOption(term *cp.Terminal, q *prompt.Question) (prompt.Response, error) {
	var blankCount = 0
	var internalMessage = ""
	for {
		// Write out messages
		if len(internalMessage) > 0 {
			fmt.Fprintf(term.Out, "%s\n\n", internalMessage)
		}
		if len(q.Prompt) > 0 {
			fmt.Fprintf(term.Out, "%s\n", q.Prompt)
		}
		for index, opt := range q.Options {
			fmt.Fprintf(term.Out, " (%d) %s\n", index+1, opt.Prompt())
			if len(opt.Validation()) > 0 {
				fmt.Fprintf(term.Out, "  ** %s\n", opt.Validation())
			}
		}
		// Reset message.
		internalMessage = ""
		ln, err := term.Basic(" # ", false)
		if err != nil {
			return prompt.RespCancel, err
		}
		if len(ln) == 0 && blankCount > 0 {
			return prompt.RespCancel, nil
		}
		if len(ln) == 0 {
			internalMessage = "Press enter again to cancel"
			blankCount++
			continue
		}
		blankCount = 0
		choice, err := strconv.ParseInt(ln, 10, 32)
		if err != nil {
			internalMessage = "Not a valid number"
			continue
		}
		index := int(choice - 1)
		if index < 0 || index >= len(q.Options) {
			internalMessage = "Not a valid choice."
			continue
		}
		opt := &q.Options[index]
		opt.Chosen = true
		if opt.Other() {
			res, err := setOther(term, q, opt)
			if err != nil {
				return prompt.RespCancel, err
			}
			if res == prompt.RespCancel {
				opt.Chosen = false
				continue
			}
		}
		return prompt.RespAnswer, nil
	}
}
