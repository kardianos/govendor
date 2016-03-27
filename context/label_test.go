// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"testing"
)

func TestLabelAnalysis(t *testing.T) {
	list := []struct {
		label  Label
		groups []labelGroup
	}{
		{
			label: Label{Source: LabelTag, Text: "v1.10-alpha"},
			groups: []labelGroup{
				{
					seq: "v1.10",
					sections: []labelSection{
						{seq: "v", number: -1, brokenBy: 0},
						{seq: "1", number: 1, brokenBy: '.'},
						{seq: "10", number: 10, brokenBy: 0},
					},
				},
				{
					seq: "alpha",
					sections: []labelSection{
						{seq: "alpha", number: -1, brokenBy: 0},
					},
				},
			},
		},
		{
			label: Label{Source: LabelTag, Text: "v1.10"},
			groups: []labelGroup{
				{
					seq: "v1.10",
					sections: []labelSection{
						{seq: "v", number: -1, brokenBy: 0},
						{seq: "1", number: 1, brokenBy: '.'},
						{seq: "10", number: 10, brokenBy: 0},
					},
				},
			},
		},
		{
			label: Label{Source: LabelTag, Text: "v1.8"},
			groups: []labelGroup{
				{
					seq: "v1.8",
					sections: []labelSection{
						{seq: "v", number: -1, brokenBy: 0},
						{seq: "1", number: 1, brokenBy: '.'},
						{seq: "8", number: 8, brokenBy: 0},
					},
				},
			},
		},
		{
			label: Label{Source: LabelTag, Text: "v1mix100"},
			groups: []labelGroup{
				{
					seq: "v1mix100",
					sections: []labelSection{
						{seq: "v", number: -1, brokenBy: 0},
						{seq: "1", number: 1, brokenBy: 0},
						{seq: "mix", number: -1, brokenBy: 0},
						{seq: "100", number: 100, brokenBy: 0},
					},
				},
			},
		},
	}

	buf := &bytes.Buffer{}
	for _, item := range list {
		analysis := &labelAnalysis{
			Label: item.label,
		}
		analysis.fillSections(buf)
		if buf.Len() != 0 {
			t.Errorf("for %q, buffer is not reset after", item.label.Text)
		}
		if len(analysis.Groups) != len(item.groups) {
			t.Errorf("for %q, got %d groups (%#v), want %d groups", item.label.Text, len(analysis.Groups), analysis.Groups, len(item.groups))
			continue
		}
		for i := range analysis.Groups {
			ag := analysis.Groups[i]
			ig := item.groups[i]
			if len(ag.sections) != len(ig.sections) {
				t.Errorf("for %q, got %d sections (%#v), want %d sections", item.label.Text, len(ag.sections), ag.sections, len(ig.sections))
				continue
			}
			for j := range ag.sections {
				if ag.sections[j] != ig.sections[j] {
					t.Errorf("for %q -> %q, got %#v, want %#v", item.label.Text, ag.seq, ag.sections[j], ig.sections[j])
				}
			}
		}
	}
}
func TestLabelOrder(t *testing.T) {
	workOn := -1
	llA := []Label{
		Label{Source: LabelTag, Text: "v1"},
		Label{Source: LabelBranch, Text: "v1"},
	}
	llB := []Label{
		Label{Source: LabelTag, Text: "v1.10-alpha"},
		Label{Source: LabelTag, Text: "v1.10-beta"},
		Label{Source: LabelTag, Text: "v1.10"},
		Label{Source: LabelTag, Text: "v1.10"},
		Label{Source: LabelTag, Text: "v1.8"},
	}
	llC := []Label{
		Label{Source: LabelTag, Text: "v1.10-alpha"},
		Label{Source: LabelTag, Text: "v1.10-beta"},
		Label{Source: LabelTag, Text: "v1.10.1-alpha"},
		Label{Source: LabelTag, Text: "v1.10.1-beta"},
		Label{Source: LabelTag, Text: "v1.10.2-alpha"},
		Label{Source: LabelTag, Text: "v1.10.2-beta"},
		Label{Source: LabelTag, Text: "v1.10.1"},
		Label{Source: LabelTag, Text: "v1.10"},
		Label{Source: LabelTag, Text: "v1.8"},
		Label{Source: LabelTag, Text: "v0.8"},
		Label{Source: LabelTag, Text: "v0.8.1"},
		Label{Source: LabelTag, Text: "v1"},
		Label{Source: LabelTag, Text: "v2.20"},
		Label{Source: LabelTag, Text: "v2"},
	}
	llD := []Label{
		Label{Source: LabelTag, Text: "v0.mix100a"},
		Label{Source: LabelTag, Text: "v1.mix100d"},
		Label{Source: LabelTag, Text: "v1.mix100e"},
		Label{Source: LabelTag, Text: "v1.mix80"},
		Label{Source: LabelTag, Text: "v2.mix200"},
	}
	llE := []Label{
		Label{Source: LabelTag, Text: "0.1"},
		Label{Source: LabelTag, Text: "1.0"},
		Label{Source: LabelTag, Text: "1.1"},
		Label{Source: LabelTag, Text: "1.1.1"},
		Label{Source: LabelTag, Text: "1.1.1-beta"},
		Label{Source: LabelTag, Text: "1.2.1-alpha"},
		Label{Source: LabelTag, Text: "1.2.1-alpha2"},
		Label{Source: LabelTag, Text: "2.0"},
	}
	list := []struct {
		version string
		labels  []Label
		find    Label
	}{
		{
			version: "v1",
			labels:  llA,
			find:    Label{Source: LabelBranch, Text: "v1"},
		},
		{
			version: "not-found",
			labels:  llA,
			find:    Label{Source: LabelNone},
		},
		{
			version: "v1",
			labels:  llB,
			find:    Label{Source: LabelTag, Text: "v1.10"},
		},
		{
			version: "v1.8",
			labels:  llB,
			find:    Label{Source: LabelTag, Text: "v1.8"},
		},
		{
			version: "v1",
			labels:  llC,
			find:    Label{Source: LabelTag, Text: "v1.10.1"},
		},
		{
			version: "v1.10.2",
			labels:  llC,
			find:    Label{Source: LabelTag, Text: "v1.10.2-beta"},
		},
		{
			version: "v1.10",
			labels:  llC,
			find:    Label{Source: LabelTag, Text: "v1.10.1"},
		},
		{
			version: "v1.10.1",
			labels:  llC,
			find:    Label{Source: LabelTag, Text: "v1.10.1"},
		},
		{
			version: "=v1",
			labels:  llC,
			find:    Label{Source: LabelTag, Text: "v1"},
		},
		{
			version: "v1",
			labels:  llD,
			find:    Label{Source: LabelTag, Text: "v1.mix100e"},
		},
		{
			version: "1",
			labels:  llE,
			find:    Label{Source: LabelTag, Text: "1.1.1"},
		},
	}
	for index, item := range list {
		if workOn >= 0 && workOn != index {
			continue
		}
		got := FindLabel(item.version, item.labels)
		if got != item.find {
			t.Errorf("For %q (index %d), got %#v, want %#v", item.version, index, got, item.find)
		}
	}
}
