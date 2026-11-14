package usn

import "testing"

func TestValid(t *testing.T) {
	for _, ok := range []string{"4MC23AI001", "4MC00AA000", "4MC19CS123"} {
		if !Valid(ok) {
			t.Errorf("%q should be a USN", ok)
		}
	}
	for _, bad := range []string{"", "4MC23AI00", "4MC23AI0011", "4mc23ai001", "4MC2AAI001", "4MC23A1001", "1MC23AI001", "4MC23AI00A", " 4MC23AI001", "4MC23AI001\n", "x"} {
		if Valid(bad) {
			t.Errorf("%q should not be a USN", bad)
		}
	}
}

func TestDept(t *testing.T) {
	if got := Dept("4MC23AI001"); got != "AI" {
		t.Errorf("Dept = %q, want AI", got)
	}
	for _, bad := range []string{"", "4MC", "4MC2", "4MC23A"} {
		if got := Dept(bad); got != "" {
			t.Errorf("Dept(%q) = %q, want empty", bad, got)
		}
	}
}
