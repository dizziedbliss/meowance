package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmNeedsTheIDTypedBack(t *testing.T) {
	cases := map[string]bool{"abc123\n": true, "  abc123  \n": true, "abc12\n": false, "y\n": false, "\n": false, "": false}
	for in, want := range cases {
		var out bytes.Buffer
		if got := confirm(strings.NewReader(in), &out, "abc123"); got != want {
			t.Errorf("confirm(%q) = %v, want %v", in, got, want)
		}
	}
}
