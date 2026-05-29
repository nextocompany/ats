package scoring

import (
	"strings"

	"github.com/nexto/hr-ats/internal/ai"
)

// Education ordinals (higher = more advanced).
const (
	eduUnknown  = 0
	eduHigh     = 1 // ม.6 / ปวช / high school
	eduDiploma  = 2 // ปวส / diploma / อนุปริญญา
	eduBachelor = 3
	eduMaster   = 4
	eduDoctor   = 5
)

// educationOrdinal maps a degree string (Thai or English) to an ordinal.
func educationOrdinal(degree string) int {
	d := strings.ToLower(degree)
	switch {
	case containsAny(d, "เอก", "doctor", "phd", "ph.d"):
		return eduDoctor
	case containsAny(d, "โท", "master"):
		return eduMaster
	case containsAny(d, "ตรี", "bachelor", "ป.ตรี", "ปริญญาตรี"):
		return eduBachelor
	case containsAny(d, "ปวส", "diploma", "อนุปริญญา"):
		return eduDiploma
	case containsAny(d, "ปวช", "ม.6", "มัธยม", "high school", "secondary"):
		return eduHigh
	default:
		return eduUnknown
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// maxEducation returns the highest education ordinal across the profile.
func maxEducation(p ai.Profile) int {
	best := eduUnknown
	for _, e := range p.Education {
		if o := educationOrdinal(e.Degree); o > best {
			best = o
		}
	}
	return best
}

// totalExperienceMonths sums experience durations.
func totalExperienceMonths(p ai.Profile) int {
	sum := 0
	for _, e := range p.Experience {
		sum += e.DurationMonths
	}
	return sum
}

// gatePassed enforces the must-have minimums (binary).
func gatePassed(p ai.Profile, jd JD) bool {
	if maxEducation(p) < jd.MinEducationLevel {
		return false
	}
	if totalExperienceMonths(p) < jd.MinExperienceMonths {
		return false
	}
	return true
}

// experienceScore scores 0–30 relative to the required minimum.
func experienceScore(months, min int) int {
	switch {
	case min > 0:
		switch {
		case months >= min*2:
			return 30
		case months >= min:
			return 20
		case months > 0:
			return 10
		default:
			return 0
		}
	default: // no minimum specified — score on absolute experience
		switch {
		case months >= 24:
			return 30
		case months >= 12:
			return 20
		case months > 0:
			return 10
		default:
			return 0
		}
	}
}

// educationScore scores 0–10 relative to the required level.
func educationScore(ord, min int) int {
	switch {
	case ord >= min+2:
		return 10
	case ord >= min+1:
		return 7
	case ord >= min:
		return 5
	default:
		return 0
	}
}

// languageScore scores 0–10 from declared languages (Thai +5, English +5).
func languageScore(p ai.Profile) int {
	score := 0
	for _, l := range p.Languages {
		name := strings.ToLower(l.Language)
		if containsAny(name, "thai", "ไทย") {
			score += 5
		}
		if containsAny(name, "english", "อังกฤษ") {
			score += 5
		}
	}
	if score > 10 {
		score = 10
	}
	return score
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
