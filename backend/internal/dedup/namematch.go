package dedup

import "strings"

// NameMatchesAny reports whether resume loosely matches at least one of the given
// candidate names. Empty candidate names are SKIPPED, not passed to
// NameLooselyMatches — which returns true for an empty argument, so feeding it a
// blank name would auto-pass and defeat the mismatch gate. Returns false when
// every candidate name is empty (caller decides whether that means "skip").
func NameMatchesAny(resume string, names ...string) bool {
	for _, n := range names {
		if n != "" && NameLooselyMatches(n, resume) {
			return true
		}
	}
	return false
}

// honorifics are stripped before comparing names (Thai + common English titles).
var honorifics = []string{"นางสาว", "นาง", "นาย", "น.ส.", "ดร.", "mr.", "mrs.", "ms.", "mr ", "mrs ", "ms "}

// NameLooselyMatches reports whether two person names plausibly refer to the same
// person. It is deliberately LENIENT (biased toward "match") so legitimate
// applicants are never falsely rejected over Thai name variants, nicknames, name
// order, titles, or OCR slips — it returns false ONLY when the names are clearly
// unrelated (share no token and are edit-distant). Used by the resume-name vs
// account-name gate; an empty name on either side returns true (nothing to judge).
func NameLooselyMatches(a, b string) bool {
	na, nb := normalizeName(a), normalizeName(b)
	if na == "" || nb == "" {
		return true
	}
	if na == nb || strings.Contains(na, nb) || strings.Contains(nb, na) {
		return true
	}
	// A shared significant token (a first or last name, allowing a 1-edit OCR slip)
	// means the same person in a different order / partial form.
	for _, x := range strings.Fields(na) {
		if len([]rune(x)) < 2 {
			continue
		}
		for _, y := range strings.Fields(nb) {
			if len([]rune(y)) < 2 {
				continue
			}
			if x == y || levenshtein(x, y) <= 1 {
				return true
			}
		}
	}
	// Whole-string fuzzy match, tolerating OCR noise proportional to length.
	tol := len([]rune(na)) / 4
	if tol < 2 {
		tol = 2
	}
	return levenshtein(na, nb) <= tol
}

// normalizeName lowercases, trims, strips a leading honorific, and collapses
// internal whitespace so name comparisons are stable across formatting.
func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, p := range honorifics {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
			break
		}
	}
	return strings.Join(strings.Fields(s), " ")
}
