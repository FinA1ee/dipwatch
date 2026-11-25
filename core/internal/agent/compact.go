package agent

import (
	"regexp"
	"strings"
)

// compact removes noise from a Playwright accessibility snapshot before it is
// excerpted: element refs on non-interactive lines, display attributes, URL query
// strings and tracking segments, repeated links, empty containers and the one- or
// two-character fragments that sites split prices into. On a real Amazon results
// page this shrinks the snapshot to about a quarter while keeping every title and price.

var (
	attrPattern     = regexp.MustCompile(` \[(?:cursor|level|active|aria-hidden|expanded|selected|checked|disabled|pressed)[^\]]*\]`)
	refPattern      = regexp.MustCompile(` \[ref=[^\]]+\]`)
	trackingSegment = regexp.MustCompile(`/ref=[^/]*$`)
	emptyContainer  = regexp.MustCompile(`^- (?:generic|group|list|listitem|link|img|separator|navigation|region|main|contentinfo|banner|article|figure|paragraph)(?: "[^"]{0,2}")?:?$`)
	// Only ASCII scraps: short CJK words such as 原价 (list price) carry meaning.
	shortFragment = regexp.MustCompile(`^- (?:text|generic): "?[\w.,:;()+/ -]{0,3}"?$`)
	// A line holding only a currency symbol, followed by number fragments: sites
	// split "¥1640.95" into "¥", "1640." and "95" for styling.
	loneCurrency   = regexp.MustCompile(`^- (?:text|generic): "?([¥￥$€£]|S\$|SGD|USD|CNY|JPY)"?$`)
	numberFragment = regexp.MustCompile(`^- (?:text|generic): "?([\d,]*\.?\d*)"?$`)
	// Refs are kept where the model might click or type.
	interactiveRole = regexp.MustCompile(`^- (?:link|button|textbox|searchbox|combobox|checkbox|tab|option)\b`)
)

func compact(snapshot string) string {
	lines := mergePriceFragments(strings.Split(snapshot, "\n"))
	out := make([]string, 0, len(lines))
	lastURL := ""
	for i, l := range lines {
		if i < headerLines { // URL, title, status
			out = append(out, l)
			continue
		}
		indent := l[:len(l)-len(strings.TrimLeft(l, " "))]
		t := strings.TrimSpace(l)

		if strings.HasPrefix(t, "- /url: ") {
			u := strings.TrimPrefix(t, "- /url: ")
			u, _, _ = strings.Cut(u, "?")
			u = trackingSegment.ReplaceAllString(u, "")
			if u == lastURL {
				continue
			}
			lastURL = u
			out = append(out, indent+"- /url: "+u)
			continue
		}

		t = attrPattern.ReplaceAllString(t, "")
		if !interactiveRole.MatchString(t) {
			t = refPattern.ReplaceAllString(t, "")
		}
		if emptyContainer.MatchString(t) || shortFragment.MatchString(t) {
			continue
		}
		out = append(out, indent+t)
	}
	return strings.Join(out, "\n")
}

// mergePriceFragments joins a lone currency line with the number fragments right
// after it, so "¥" "1640." "95" becomes "¥1640.95" before short fragments are dropped.
func mergePriceFragments(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		t := refPattern.ReplaceAllString(strings.TrimSpace(lines[i]), "")
		m := loneCurrency.FindStringSubmatch(t)
		if m == nil {
			out = append(out, lines[i])
			continue
		}
		var number strings.Builder
		j := i + 1
		for ; j < len(lines) && j <= i+3; j++ {
			f := numberFragment.FindStringSubmatch(refPattern.ReplaceAllString(strings.TrimSpace(lines[j]), ""))
			if f == nil || f[1] == "" {
				break
			}
			number.WriteString(f[1])
		}
		if number.Len() == 0 {
			out = append(out, lines[i])
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
		out = append(out, indent+"- text: "+m[1]+strings.TrimSuffix(number.String(), "."))
		i = j - 1
	}
	return out
}
