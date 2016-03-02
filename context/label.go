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

type labelSection struct {
	seq      string
	number   int64
	brokenBy rune
}

type labelAnalysis struct {
	Label    Label
	Sections []labelSection
}

func (item *labelAnalysis) fillSections(buf *bytes.Buffer) {
	previousNumber := false
	number := false

	isBreak := func(r rune) bool {
		return r == '.' || r == '-'
	}
	add := func(r rune) {
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
			item.Sections = append(item.Sections, labelSection{
				seq:      sVal,
				number:   value,
				brokenBy: r,
			})
		}
	}
	for index, r := range item.Label.Text {
		number = unicode.IsNumber(r)
		different := number != previousNumber && index > 0
		previousNumber = number
		if isBreak(r) {
			add(r)
			continue
		}
		if different {
			add(r)
			buf.WriteRune(r)
			continue
		}
		buf.WriteRune(r)
	}
	add(0)
	buf.Reset()
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
			fmt.Printf(f, a)
		}
	}
	a := l[i]
	b := l[j]
	swap := false
	min := a
	max := b
	if len(max.Sections) < len(min.Sections) {
		min, max = max, min
		swap = true
	}
	df(":: %s vs %s ::\n", a.Label.Text, b.Label.Text)
	for i := range min.Sections {
		sa := a.Sections[i]
		sb := b.Sections[i]

		// Sort "-alpha" and "-beta" tags down.
		if sa.brokenBy != sb.brokenBy {
			if sa.brokenBy == '-' {
				return false
			}
			if sb.brokenBy == '-' {
				return true
			}
		}

		if sa.number != sb.number {
			df("PT A")
			return sa.number > sb.number
		}
		if sa.seq != sb.seq {
			df("PT B")
			return sa.seq > sb.seq
		}
	}
	if len(a.Sections) == len(b.Sections) {
		if a.Label.Source != b.Label.Source {
			if a.Label.Source == LabelBranch {
				df("PT C")
				return true
			}
		}
		df("PT D")
		return false
	}
	var bb rune
	if len(min.Sections) == 0 {
		bb = max.Sections[0].brokenBy
	} else {
		bb = max.Sections[len(min.Sections)-1].brokenBy
	}
	if bb == '-' {
		df("PT E")
		return !swap
	}
	df("PT F")
	return swap
}

// FindLabel matches a single label from a list of labels, given a version.
// If the returning label.Source is LabelNone, then no labels match.
func FindLabel(version string, labels []Label) Label {
	//Versions in the package-spec are a special prefix matching that
	//checks vcs branches and tags.
	//When "version = v1" then the following would all match: v1.4, v1.8, v1.12.
	//The following would *not* match: v10, foo-v1.4, v1-40
	//
	//After matching acceptable labels, they must be sorted and a single label
	//returned. Of the following: "v1.4, v1.8, v1.12, v1.12-beta --> v1.12 would
	//be choosen.
	//
	//There is no precedence between branches and tags, they are both searched for
	//labels and sorted all together to find the correct match. In case of two
	//labels with exactly the same, one from branch, one from tag, choose the branch.
	//
	//In the go repo: "version = release-branch.go1" would currently return
	//the branch: "release-branch.go1.6".

	/*
		Match version string prefix.
			If no simple prefix match found, reject.

		Next character after match must be end of string, "." or "-".
			If not, reject.

		Push into list sequences of letters, sequences of numbers, broken by "." or "-" as well.
		Keep track of how it is broken up. (letter number transition, "." or "-", or end of string.

		StableSort list of sequences, choose top one.
	*/

	list := make([]*labelAnalysis, 0, 6)

	for _, label := range labels {
		if strings.HasPrefix(label.Text, version) == false {
			continue
		}
		remain := strings.TrimPrefix(label.Text, version)
		if len(remain) > 0 {
			next := remain[0]
			if next != '.' && next != '-' {
				continue
			}
		}
		list = append(list, &labelAnalysis{
			Label:    label,
			Sections: make([]labelSection, 0, 6),
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
