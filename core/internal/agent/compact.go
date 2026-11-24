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
	shortFragment   = regexp.MustCompile(`^- (?:text|generic): "?.{0,3}"?$`)
	// Refs are kept where the model might click or type.
	interactiveRole = regexp.MustCompile(`^- (?:link|button|textbox|searchbox|combobox|checkbox|tab|option)\b`)
)

func compact(snapshot string) string {
	lines := strings.Split(snapshot, "\n")
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
