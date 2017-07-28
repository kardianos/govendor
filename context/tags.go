// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"strings"
)

// Build tags come in the format "tagA tagB,tagC" -> "taga OR (tagB AND tagC)"
// File tags compose with this as "ftag1 AND ftag2 AND (<build-tags>)".
// However in govendor all questions are reversed. Rather than asking
// "What should be built?" we ask "What should be ignored?".

type logical struct {
	and bool
	tag []logicalTag
	sub []logical
}
type logicalTag struct {
	not bool
	tag string
}

func (lt logicalTag) match(lt2 logicalTag) bool {
	// A logicalTag is a match if
	// "tag == itag" or "!tag == !itag" or
	// "tag != !itag" or "!tag != itag"
	if lt.not || lt2.not {
		return false
	}
	if lt.tag == lt2.tag {
		return true
	}

	if lt.not == lt2.not {
		return lt.tag == lt2.tag
	}
	return lt.tag != lt2.tag
}

func (lt logicalTag) conflict(lt2 logicalTag) bool {
	return lt.tag == lt2.tag && lt.not != lt2.not
}

func (lt logicalTag) String() string {
	if lt.not {
		return "!" + lt.tag
	}
	return lt.tag
}

func newLogicalTag(tag string) logicalTag {
	lt := logicalTag{}
	lt.not = strings.HasPrefix(tag, "!")
	lt.tag = strings.TrimPrefix(tag, "!")
	return lt
}

func (l logical) empty() bool {
	if len(l.tag) > 0 {
		return false
	}
	for _, sub := range l.sub {
		if !sub.empty() {
			return false
		}
	}
	return true
}

func (l logical) ignored(ignoreTags []logicalTag) bool {
	// A logical is ignored if ANY AND conditions match or ALL OR conditions match.
	if len(ignoreTags) == 0 {
		return false
	}
	if l.empty() {
		return !l.and
	}
	if l.and {
		// Must have all tags in ignoreTags to be ignored.
		for _, t := range l.tag {
			for _, it := range ignoreTags {
				if t.match(it) {
					return true
				}
			}
		}

		// Must ignore all sub-logicals to be ignored.
		for _, sub := range l.sub {
			if sub.ignored(ignoreTags) {
				return true
			}
		}

		return false
	}

	hasOne := false
	// OR'ing the pieces together.
	// Must have at least one tag in ignoreTags to be ignored.
	for _, t := range l.tag {
		hasOne = true
		hasIgnoreTag := false
		for _, it := range ignoreTags {
			if t.match(it) {
				hasIgnoreTag = true
				break
			}
		}
		if !hasIgnoreTag {
			return false
		}
	}

	// Must have at least one sub section be ignored to be ignored.
	for _, sub := range l.sub {
		hasOne = true
		if !sub.ignored(ignoreTags) {
			return false
		}
	}
	return hasOne
}

func (l logical) conflict(lt logicalTag) bool {
	for _, t := range l.tag {
		if t.conflict(lt) {
			return true
		}
	}
	for _, s := range l.sub {
		if s.conflict(lt) {
			return true
		}
	}
	return false
}

func (l logical) String() string {
	buf := bytes.Buffer{}
	if l.and {
		buf.WriteString(" AND (")
	} else {
		buf.WriteString(" OR (")
	}

	for index, tag := range l.tag {
		if index != 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(tag.String())
	}
	for index, sub := range l.sub {
		if index != 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(sub.String())
	}

	buf.WriteRune(')')
	return buf.String()
}

type TagSet struct {
	// ignore comes from a special build tag "ignore".
	ignore bool

	root logical
}

func (ts *TagSet) String() string {
	if ts == nil {
		return "(nil)"
	}
	if ts.ignore {
		return "ignore"
	}
	return ts.root.String()
}

func (ts *TagSet) IgnoreItem(ignoreList ...string) bool {
	if ts == nil {
		return false
	}
	if ts.ignore {
		return true
	}
	for _, fileTag := range ts.root.tag {
		for _, buildTag := range ts.root.sub {
			if buildTag.conflict(fileTag) {
				return true
			}
		}
	}
	ts.root.and = true
	ignoreTags := make([]logicalTag, len(ignoreList))
	for i := 0; i < len(ignoreList); i++ {
		ignoreTags[i] = newLogicalTag(ignoreList[i])
	}
	return ts.root.ignored(ignoreTags)
}

func (ts *TagSet) AddFileTag(tag string) {
	if ts == nil {
		return
	}
	ts.root.and = true
	ts.root.tag = append(ts.root.tag, newLogicalTag(tag))
}
func (ts *TagSet) AddBuildTags(tags string) {
	if ts == nil {
		return
	}
	ts.root.and = true
	if len(ts.root.sub) == 0 {
		ts.root.sub = append(ts.root.sub, logical{})
	}
	buildlogical := &ts.root.sub[0]
	ss := strings.Fields(tags)
	for _, s := range ss {
		if s == "ignore" {
			ts.ignore = true
			continue
		}
		if !strings.ContainsRune(s, ',') {
			buildlogical.tag = append(buildlogical.tag, newLogicalTag(s))
			continue
		}
		sub := logical{and: true}
		for _, and := range strings.Split(s, ",") {
			sub.tag = append(sub.tag, newLogicalTag(and))
		}
		buildlogical.sub = append(buildlogical.sub, sub)

	}
}
