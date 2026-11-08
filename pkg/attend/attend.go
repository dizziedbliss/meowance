// Package attend holds the bunk-math: pure functions over (attended, conducted)
// that tell a student how many classes they can still skip, or must attend, to
// stay on the right side of an attendance threshold.
//
// All functions depend ONLY on attended + conducted + the target fraction, so
// they never rely on the semester's total planned classes (which MCE's portal
// pads retroactively and cannot be trusted mid-semester). See the project plan's
// "Data-quality caveats" for why that matters.
package attend

import "math"

// epsilon absorbs float rounding so exact integer boundaries (e.g. 3.0000001 or
// 2.9999999) floor/ceil to the mathematically correct value.
const epsilon = 1e-9

// Ratio returns attended/conducted as a fraction in [0,1]. Zero conducted ⇒ 0.
func Ratio(attended, conducted int) float64 {
	if conducted <= 0 {
		return 0
	}
	return float64(attended) / float64(conducted)
}

// CanBunk reports how many of the upcoming classes you may skip — attending NONE
// of them — while still ending at or above target. It answers "how many more can
// I miss right now?" and is clamped at 0 (a negative raw value means you're
// already below target, so there is nothing to spare — use Recover instead).
//
// Derivation: want attended/(conducted+b) >= target
//
//	⇒ b <= attended/target - conducted.
func CanBunk(attended, conducted int, target float64) int {
	if target <= 0 {
		return 0
	}
	raw := float64(attended)/target - float64(conducted)
	b := int(math.Floor(raw + epsilon))
	if b < 0 {
		return 0
	}
	return b
}

// Recover reports the minimum number of consecutive classes you must attend
// (each one also conducted) to climb back to at or above target. It returns 0
// when you are already at or above target.
//
// Derivation: want (attended+k)/(conducted+k) >= target
//
//	⇒ k >= (target*conducted - attended) / (1 - target).
func Recover(attended, conducted int, target float64) int {
	if target >= 1 { // reaching 100% is only possible if already there
		if Ratio(attended, conducted) >= 1 {
			return 0
		}
		return -1 // unreachable; callers treat <0 as "impossible"
	}
	if Ratio(attended, conducted) >= target {
		return 0
	}
	k := (target*float64(conducted) - float64(attended)) / (1 - target)
	r := int(math.Ceil(k - epsilon))
	if r < 0 {
		return 0
	}
	return r
}

// Status bundles the two headline answers for one course at one threshold.
type Status struct {
	Target  float64 // e.g. 0.75
	Below   bool    // currently under target?
	CanBunk int     // classes skippable now (0 if Below)
	Recover int     // consecutive must-attends to reach target (0 if not Below)
}

// At computes the Status for a course at a single threshold.
func At(attended, conducted int, target float64) Status {
	below := Ratio(attended, conducted) < target
	return Status{
		Target:  target,
		Below:   below,
		CanBunk: CanBunk(attended, conducted, target),
		Recover: Recover(attended, conducted, target),
	}
}
