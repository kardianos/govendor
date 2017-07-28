// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type LabelSource byte

const (
	LabelNone LabelSource = iota
	LabelBranch
	LabelTag
)

func (ls LabelSource) String() string {
	switch ls {
	default:
		panic("unknown label source")
	case LabelNone:
		return "none"
	case LabelBranch:
		return "branch"
	case LabelTag:
		return "tag"
	}
}

type Label struct {
	Text   string
	Source LabelSource
}

func (l Label) String() string {
	return fmt.Sprintf("[%s]%s", l.Source, l.Text)
}

type labelGroup struct {
	seq      string
	sections []labelSection
}

type labelSection struct {
	seq      string
	number   int64
	brokenBy rune
}

type labelAnalysis struct {
	Label  Label
	Groups []labelGroup
}

func (item *labelAnalysis) fillSections(buf *bytes.Buffer) {
	previousNumber := false
	number := false

	isBreak := func(r rune) bool {
		return r == '.'
	}
	add := func(r rune, group *labelGroup) {
		if buf.Len() > 0 {
			sVal := buf.String()
			buf.Reset()
			value, err := strconv.ParseInt(sVal, 10, 64)
			if err != nil {
				value = -1
			}
			if isBreak(r) == false {
				r = 0
			}
			group.sections = append(group.sections, labelSection{
				seq:      sVal,
				number:   value,
				brokenBy: r,
			})
		}
	}
	for _, groupText := range strings.Split(item.Label.Text, "-") {
		group := labelGroup{
			seq: groupText,
		}
		for index, r := range groupText {
			number = unicode.IsNumber(r)
			different := number != previousNumber && index > 0
			previousNumber = number
			if isBreak(r) {
				add(r, &group)
				continue
			}
			if different {
				add(r, &group)
				buf.WriteRune(r)
				continue
			}
			buf.WriteRune(r)
		}
		add(0, &group)
		buf.Reset()
		item.Groups = append(item.Groups, group)
	}
}

type labelAnalysisList []*labelAnalysis

func (l labelAnalysisList) Len() int {
	return len(l)
}
func (l labelAnalysisList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l labelAnalysisList) Less(i, j int) bool {
	const debug = false
	df := func(f string, a ...interface{}) {
		if debug {
			fmt.Printf(f, a...)
		}
	}
	a := l[i]
	b := l[j]

	// Want to return the *smaller* of the two group counts.
	if len(a.Groups) != len(b.Groups) {
		return len(a.Groups) < len(b.Groups)
	}

	gct := len(a.Groups)
	if gct > len(b.Groups) {
		gct = len(b.Groups)
	}

	df(":: %s vs %s ::\n", a.Label.Text, b.Label.Text)
	for ig := 0; ig < gct; ig++ {
		ga := a.Groups[ig]
		gb := b.Groups[ig]

		if ga.seq == gb.seq {
			df("pt 1 %q\n", ga.seq)
			continue
		}

		ct := len(ga.sections)
		if ct > len(gb.sections) {
			ct = len(gb.sections)
		}

		// Compare common sections.
		for i := 0; i < ct; i++ {
			sa := ga.sections[i]
			sb := gb.sections[i]

			// Sort each section by number and alpha.
			if sa.number != sb.number {
				df("PT A\n")
				return sa.number > sb.number
			}
			if sa.seq != sb.seq {
				df("PT B\n")
				return sa.seq > sb.seq
			}
		}

		// Sections that we can compare are equal, we want
		// the longer of the two sections if lengths un-equal.
		if len(ga.sections) != len(gb.sections) {
			return len(ga.sections) > len(gb.sections)
		}
	}
	// At this point we have same number of groups and same number
	// of sections. We can assume the labels are the same.
	// Check to see if the source of the label is different.
	if a.Label.Source != b.Label.Source {
		if a.Label.Source == LabelBranch {
			df("PT C\n")
			return true
		}
	}
	// We ran out of things to check. Assume one is not "less" than the other.
	df("PT D\n")
	return false
}

// FindLabel matches a single label from a list of labels, given a version.
// If the returning label.Source is LabelNone, then no labels match.
//
// Labels are first broken into sections separated by "-". Shortest wins.
// If they have the same number of above sections, then they are compared
// further. Number sequences are treated as numbers. Numbers do not need a
// separator. The "." is a break point as well.
func FindLabel(version string, labels []Label) Label {
	list := make([]*labelAnalysis, 0, 6)

	exact := strings.HasPrefix(version, "=")
	version = strings.TrimPrefix(version, "=")

	for _, label := range labels {
		if exact {
			if label.Text == version {
				return label
			}
			continue
		}
		if strings.HasPrefix(label.Text, version) == false {
			continue
		}
		remain := strings.TrimPrefix(label.Text, version)
		if len(remain) > 0 {
			next := remain[0]
			// The stated version must either be the full label,
			// followed by a "." or "-".
			if next != '.' && next != '-' {
				continue
			}
		}
		list = append(list, &labelAnalysis{
			Label:  label,
			Groups: make([]labelGroup, 0, 3),
		})
	}
	if len(list) == 0 {
		return Label{Source: LabelNone}
	}

	buf := &bytes.Buffer{}
	for _, item := range list {
		item.fillSections(buf)
		buf.Reset()
	}
	sort.Sort(labelAnalysisList(list))
	return list[0].Label
}
