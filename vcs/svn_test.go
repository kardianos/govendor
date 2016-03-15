package vcs

import "testing"

func TestSVNInfo(t *testing.T) {
	var err error
	var info = &VcsInfo{}
	var output = []byte(`<?xml version="1.0" encoding="UTF-8"?>
<info>
	<entry
	   kind="dir"
	   path="."
	   revision="1735175">
		<url>http://svn.apache.org/repos/asf/lenya/trunk</url>
		<relative-url>^/lenya/trunk</relative-url>
		<repository>
			<root>http://svn.apache.org/repos/asf</root>
			<uuid>13f79535-47bb-0310-9956-ffa450edef68</uuid>
		</repository>
		<wc-info>
			<wcroot-abspath>/home/daniel/src/test/test-svn/trunk</wcroot-abspath>
			<schedule>normal</schedule>
			<depth>infinity</depth>
		</wc-info>
		<commit
		   revision="1175731">
			<author>florent</author>
			<date>2011-09-26T09:07:59.663459Z</date>
		</commit>
	</entry>
</info>
`)

	svn := VcsSvn{}
	err = svn.parseInfo(output, info)
	if err != nil {
		t.Fatal(err)
	}
	if info.Revision != "1175731" {
		t.Error("revision inforrect")
	}
	if info.RevisionTime.Year() != 2011 {
		t.Error("time incorrect")
	}
}
