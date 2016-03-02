package context

import (
	"bytes"
	"testing"
)

func TestLabelAnalysis(t *testing.T) {
	list := []struct {
		label    Label
		sections []labelSection
	}{
		{
			label: Label{Source: LabelTag, Text: "v1.10-alpha"},
			sections: []labelSection{
				{seq: "v", number: -1, brokenBy: 0},
				{seq: "1", number: 1, brokenBy: '.'},
				{seq: "10", number: 10, brokenBy: '-'},
				{seq: "alpha", number: -1, brokenBy: 0},
			},
		},
		{
			label: Label{Source: LabelTag, Text: "v1.10"},
			sections: []labelSection{
				{seq: "v", number: -1, brokenBy: 0},
				{seq: "1", number: 1, brokenBy: '.'},
				{seq: "10", number: 10, brokenBy: 0},
			},
		},
		{
			label: Label{Source: LabelTag, Text: "v1.8"},
			sections: []labelSection{
				{seq: "v", number: -1, brokenBy: 0},
				{seq: "1", number: 1, brokenBy: '.'},
				{seq: "8", number: 8, brokenBy: 0},
			},
		},
		{
			label: Label{Source: LabelTag, Text: "v1mix100"},
			sections: []labelSection{
				{seq: "v", number: -1, brokenBy: 0},
				{seq: "1", number: 1, brokenBy: 0},
				{seq: "mix", number: -1, brokenBy: 0},
				{seq: "100", number: 100, brokenBy: 0},
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
		if len(analysis.Sections) != len(item.sections) {
			t.Errorf("for %q, got %d sections (%#v), want %d sections", item.label.Text, len(analysis.Sections), analysis.Sections, len(item.sections))
			continue
		}
		for i := range analysis.Sections {
			if analysis.Sections[i] != item.sections[i] {
				t.Errorf("for %q, got %#v, want %#v", item.label.Text, analysis.Sections[i], item.sections[i])
			}
		}
	}
}
func TestLabelOrder(t *testing.T) {
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
		Label{Source: LabelTag, Text: "v1.10.1"},
		Label{Source: LabelTag, Text: "v1.10"},
		Label{Source: LabelTag, Text: "v1.8"},
	}
	llD := []Label{
		Label{Source: LabelTag, Text: "v1.mix100d"},
		Label{Source: LabelTag, Text: "v1.mix100e"},
		Label{Source: LabelTag, Text: "v1.mix80"},
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
			find:    Label{Source: LabelTag, Text: "v1.10.2-alpha"},
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
			version: "v1",
			labels:  llD,
			find:    Label{Source: LabelTag, Text: "v1.mix100e"},
		},
	}
	for index, item := range list {
		got := FindLabel(item.version, item.labels)
		if got != item.find {
			t.Errorf("For %q (index %d), got %#v, want %#v", item.version, index, got, item.find)
		}
	}
}
