package attend

import "testing"

// Real numbers pulled from 4MC00AA000's dashboard (2026-11-08), hand-verified in
// the project plan. These lock the formulas against ground truth.
func TestRealData(t *testing.T) {
	cases := []struct {
		name             string
		att, con         int
		canBunk75, rec75 int
		canBunk85, rec85 int
	}{
		//               @75: bunk/recover      @85: bunk/recover
		{"23AI701 20/25", 20, 25, 1, 0, 0, 9},
		{"23AI702 22/27", 22, 27, 2, 0, 0, 7},
		{"23AI703 25/30", 25, 30, 3, 0, 0, 4},
		{"23AI704D 10/14", 10, 14, 0, 2, 0, 13}, // below 75 AND 85
		{"23OEEC71 5/5", 5, 5, 1, 0, 0, 0},      // 100%: 1 spare @75, safe @85
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CanBunk(c.att, c.con, 0.75); got != c.canBunk75 {
				t.Errorf("CanBunk@75 = %d, want %d", got, c.canBunk75)
			}
			if got := Recover(c.att, c.con, 0.75); got != c.rec75 {
				t.Errorf("Recover@75 = %d, want %d", got, c.rec75)
			}
			if got := CanBunk(c.att, c.con, 0.85); got != c.canBunk85 {
				t.Errorf("CanBunk@85 = %d, want %d", got, c.canBunk85)
			}
			if got := Recover(c.att, c.con, 0.85); got != c.rec85 {
				t.Errorf("Recover@85 = %d, want %d", got, c.rec85)
			}
		})
	}
}

// Skipping exactly CanBunk classes must stay >= target; one more must drop below.
func TestCanBunkIsTight(t *testing.T) {
	for _, tc := range []struct {
		att, con int
		target   float64
	}{{20, 25, 0.75}, {22, 27, 0.75}, {25, 30, 0.75}, {5, 5, 0.75}, {5, 5, 0.85}} {
		b := CanBunk(tc.att, tc.con, tc.target)
		if got := Ratio(tc.att, tc.con+b); got < tc.target {
			t.Errorf("att=%d con=%d t=%.2f: after %d bunks ratio %.4f < target", tc.att, tc.con, tc.target, b, got)
		}
		if got := Ratio(tc.att, tc.con+b+1); got >= tc.target {
			t.Errorf("att=%d con=%d t=%.2f: %d+1 bunks still >= target (%.4f), not tight", tc.att, tc.con, tc.target, b, got)
		}
	}
}

// Attending exactly Recover classes must reach >= target; one fewer must stay below.
func TestRecoverIsTight(t *testing.T) {
	for _, tc := range []struct {
		att, con int
		target   float64
	}{{20, 25, 0.85}, {22, 27, 0.85}, {25, 30, 0.85}, {10, 14, 0.75}, {10, 14, 0.85}} {
		k := Recover(tc.att, tc.con, tc.target)
		if got := Ratio(tc.att+k, tc.con+k); got < tc.target {
			t.Errorf("att=%d con=%d t=%.2f: after %d attends ratio %.4f < target", tc.att, tc.con, tc.target, k, got)
		}
		if k > 0 {
			if got := Ratio(tc.att+k-1, tc.con+k-1); got >= tc.target {
				t.Errorf("att=%d con=%d t=%.2f: %d-1 attends already >= target (%.4f), not tight", tc.att, tc.con, tc.target, k, got)
			}
		}
	}
}

func TestEdgeCases(t *testing.T) {
	if got := CanBunk(0, 0, 0.75); got != 0 {
		t.Errorf("0/0 CanBunk = %d, want 0", got)
	}
	if got := Recover(0, 0, 0.75); got != 0 {
		t.Errorf("0/0 Recover = %d, want 0", got)
	}
	// exactly on the threshold counts as satisfied: 15/20 == 75%
	if got := Recover(15, 20, 0.75); got != 0 {
		t.Errorf("15/20 Recover@75 = %d, want 0", got)
	}
	if got := CanBunk(15, 20, 0.75); got != 0 {
		t.Errorf("15/20 CanBunk@75 = %d, want 0", got)
	}
}
