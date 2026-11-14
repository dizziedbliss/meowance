package notes

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestLabelIsStablePerNote(t *testing.T) {
	if Label("note-1", "CS") != Label("note-1", "CS") {
		t.Error("the same note must always get the same label")
	}
}

func TestLabelUsesTheDeptInLowercase(t *testing.T) {
	seen := false
	for i := 0; i < 200; i++ {
		l := Label(fmt.Sprint("n", i), "AI")
		if strings.Contains(l, "{dept}") {
			t.Fatalf("placeholder left in %q", l)
		}
		if strings.Contains(l, "ai ") {
			seen = true
		}
	}
	if !seen {
		t.Error("no label ever mentioned the dept")
	}
}

func TestLabelFallsBackWhenTheDeptIsUnknown(t *testing.T) {
	for i := 0; i < 200; i++ {
		l := Label(fmt.Sprint("n", i), "")
		if strings.Contains(l, "{dept}") || strings.Contains(l, "  ") {
			t.Fatalf("bad fallback label %q", l)
		}
	}
}

func TestLabelPoolIsVariedAndSane(t *testing.T) {
	if len(labelPool) < 12 {
		t.Fatalf("only %d labels: not funny enough", len(labelPool))
	}
	used := map[string]bool{}
	for i := 0; i < 400; i++ {
		used[Label(fmt.Sprint("note-", i), "CS")] = true
	}
	if len(used) < len(labelPool)/2 {
		t.Errorf("400 notes used only %d different labels", len(used))
	}
	seen := map[string]bool{}
	for _, l := range labelPool {
		if strings.TrimSpace(l) == "" || seen[l] {
			t.Errorf("empty or duplicate label %q", l)
		}
		seen[l] = true
	}
}

func TestLabelUsesTheRightArticleForTheDept(t *testing.T) {
	// a dept is read letter by letter: "ai" and "me" (em-ee) start with a vowel sound, "cs" and "vl" don't
	wrong := map[string]*regexp.Regexp{
		"AI": regexp.MustCompile(`\ba ai\b`), "EC": regexp.MustCompile(`\ba ec\b`), "ME": regexp.MustCompile(`\ba me\b`),
		"IS": regexp.MustCompile(`\ba is\b`), "EE": regexp.MustCompile(`\ba ee\b`), "": regexp.MustCompile(`\ba mce\b`),
		"CS": regexp.MustCompile(`\ban cs\b`), "VL": regexp.MustCompile(`\ban vl\b`),
	}
	for dept, bad := range wrong {
		for i := 0; i < 300; i++ {
			if l := Label(fmt.Sprint("n", i), dept); bad.MatchString(l) {
				t.Fatalf("dept %q: %q has the wrong article", dept, l)
			}
		}
	}
	seen := false
	for i := 0; i < 300; i++ {
		if strings.Contains(Label(fmt.Sprint("n", i), "AI"), "an ai ") {
			seen = true
		}
	}
	if !seen {
		t.Error("no AI label ever read 'an ai ...'")
	}
}
