// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package prompt prompts user for feedback.
package prompt

import (
	"fmt"
)

type Option struct {
	key        interface{}
	prompt     string
	validation string
	other      bool

	Chosen bool   // Set to true if chosen.
	Value  string // Value used if chosen and option is "other".
}

type OptionType byte

const (
	TypeSelectOne      OptionType = iota // Allow user to choose single option.
	TypeSelectMultiple                   // Allow user to choose multiple options.
)

type Response byte

const (
	RespAnswer Response = iota
	RespCancel
)

func NewOption(key interface{}, prompt string, other bool) Option {
	return Option{key: key, prompt: prompt, other: other}
}

func (opt Option) Key() interface{} {
	return opt.key
}
func (opt Option) Prompt() string {
	return opt.prompt
}
func (opt Option) Other() bool {
	return opt.other
}
func (opt Option) Validation() string {
	return opt.validation
}
func (opt Option) String() string {
	if opt.other {
		return opt.Value
	}
	return fmt.Sprintf("%v", opt.key)
}

func ValidateOption(opt Option, validation string) Option {
	return Option{
		key:    opt.key,
		prompt: opt.prompt,
		other:  opt.other,

		validation: validation,

		Chosen: opt.Chosen,
		Value:  opt.Value,
	}
}

type Question struct {
	Error   string
	Prompt  string
	Type    OptionType
	Options []Option
}

func (q *Question) AnswerMultiple(must bool) []*Option {
	ans := []*Option{}
	for i := range q.Options {
		o := &q.Options[i]
		if o.Chosen {
			ans = append(ans, o)
		}
	}
	if must && len(ans) == 0 {
		panic("If no option is chosen, response must be cancelled")
	}
	return ans
}

func (q *Question) AnswerSingle(must bool) *Option {
	var ans *Option
	if q.Type != TypeSelectOne {
		panic("Question Type should match answer type")
	}
	found := false
	for i := range q.Options {
		o := &q.Options[i]
		if found && o.Chosen {
			panic("Must only respond with single option")
		}
		if o.Chosen {
			found = true
			ans = o
		}
	}
	if must && !found {
		panic("If no option is chosen, response must be cancelled")
	}
	return ans
}

type Prompt interface {
	Ask(q *Question) (Response, error)
}
