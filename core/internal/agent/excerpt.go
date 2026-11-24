package agent

import (
	"strings"
)

// A results page snapshot can be 200,000+ characters, mostly navigation and
// footers. excerpt keeps the page header and a window of lines after every line
// that names the product, where a listing's price, rating and stock usually are.
// Repeated links inside a window are collapsed. If nothing names the product, it
// returns the start of the page so the model can still scroll or search it.

const (
	headerLines   = 6   // page URL, title and status at the top
	linesBefore   = 4   // a card's image link or brand line comes just before its title
	linesAfter    = 30  // and its price comes within this many lines after
	maxLineLen    = 200 // long lines are mostly URLs with tracking parameters
	maxURLsPerHit = 1   // keep one link per title, not every repeat of it
	maxWindowLen  = 1_500
)

// Excerpt is exported for the smoke tool.
func Excerpt(snapshot, product string, limit int) string {
	return excerpt(compact(snapshot), product, limit)
}

func excerpt(snapshot, product string, limit int) string {
	if len(snapshot) <= limit {
		return snapshot
	}
	lines := strings.Split(snapshot, "\n")
	for i, l := range lines {
		if len(l) > maxLineLen {
			lines[i] = truncateLine(l, maxLineLen)
		}
	}
	terms := productTerms(product)
	head := min(headerLines, len(lines))

	var b strings.Builder
	b.WriteString("[Excerpt of a long page: lines around product titles only]\n")
	for _, l := range lines[:head] {
		b.WriteString(l)
		b.WriteByte('\n')
	}

	written := make([]bool, len(lines))
	for i := range head {
		written[i] = true
	}
	hits := 0
	for i, l := range lines {
		if written[i] || !isTitleLine(l, terms) {
			continue
		}
		var w strings.Builder
		urls := 0
		for j := max(head, i-linesBefore); j <= min(len(lines)-1, i+linesAfter); j++ {
			if written[j] {
				continue
			}
			if j > i && isTitleLine(lines[j], terms) {
				break // the next listing starts here and gets its own window
			}
			written[j] = true
			if isURLLine(lines[j]) {
				if urls >= maxURLsPerHit {
					continue
				}
				urls++
			}
			if w.Len()+len(lines[j]) > maxWindowLen {
				break
			}
			w.WriteString(lines[j])
			w.WriteByte('\n')
		}
		if b.Len()+w.Len()+4 > limit {
			continue
		}
		hits++
		b.WriteString("  …\n")
		b.WriteString(w.String())
	}

	if hits == 0 {
		b.WriteString("[No line names the product; showing the start of the page]\n")
		for _, l := range lines[head:] {
			if b.Len()+len(l)+1 > limit {
				break
			}
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// isTitleLine reports whether a line names the product as listing text: every
// distinctive word of the product appears. URLs, the search box and filters
// repeat the query, so they do not count.
func isTitleLine(l string, terms []string) bool {
	if len(terms) == 0 || isURLLine(l) {
		return false
	}
	t := strings.ToLower(strings.TrimSpace(l))
	for _, ui := range []string{"searchbox", "combobox", "textbox"} {
		if strings.Contains(t, ui) {
			return false
		}
	}
	for _, term := range terms {
		if !strings.Contains(t, term) {
			return false
		}
	}
	return true
}

func isURLLine(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "- /url:") }

// productTerms are the distinctive words of the product name, e.g. "rtx", "5090".
func productTerms(product string) []string {
	var terms []string
	for _, f := range strings.Fields(strings.ToLower(product)) {
		if len([]rune(f)) >= 3 || strings.ContainsAny(f, "0123456789") {
			terms = append(terms, f)
		}
	}
	return terms
}

func truncateLine(l string, n int) string {
	cut := n
	for cut > 0 && (l[cut]&0xC0) == 0x80 {
		cut--
	}
	return l[:cut] + "…"
}
