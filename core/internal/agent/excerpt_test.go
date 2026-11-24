package agent

import (
	"fmt"
	"strings"
	"testing"
)

// listing mimics one Amazon-style result card: repeated long links, a title, rating
// and delivery lines, and a price split into fragments.
func listing(i int, title, price string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "        - listitem [ref=l%d]:\n          - generic [ref=g%d]:\n", i, i)
	for range 3 {
		fmt.Fprintf(&b, "            - link [ref=a%d] [cursor=pointer]:\n              - /url: /dp/B0%d/ref=sr_1_%d?dib=%s\n", i, i, i, strings.Repeat("x", 300))
	}
	fmt.Fprintf(&b, "            - heading \"%s\" [level=2] [ref=h%d]\n", title, i)
	for k := range 12 {
		fmt.Fprintf(&b, "            - generic [ref=s%d_%d]: rating and delivery text %d\n", i, k, k)
	}
	fmt.Fprintf(&b, "            - generic [ref=p%d]: %s\n              - text: \"12\"\n              - generic: .\n", i, price)
	return b.String()
}

func longPage(cards string) string {
	var b strings.Builder
	b.WriteString("### Page\n- Page URL: https://www.amazon.co.jp/s?k=RTX+5090\n- Page Title: Amazon.co.jp : RTX 5090\n### Snapshot\n```yaml\n- generic [ref=e1]:\n")
	for i := range 2000 {
		fmt.Fprintf(&b, "    - link \"Navigation item %d\" [ref=n%d]\n", i, i)
	}
	b.WriteString("    - list [ref=results]:\n")
	b.WriteString(cards)
	for i := range 2000 {
		fmt.Fprintf(&b, "    - link \"Footer item %d\" [ref=f%d]\n", i, i)
	}
	return b.String()
}

func TestExcerptKeepsListingsFromALongPage(t *testing.T) {
	page := longPage(listing(1, "MSI GeForce RTX 5090 32G VENTUS 3X OC", "SGD 3,488.00") +
		listing(2, "ASUS ROG Astral RTX 5090", "SGD 4,100.00"))
	got := excerpt(compact(page), "RTX 5090", 4_000)

	if len(got) > 4_300 {
		t.Fatalf("excerpt is %d characters, limit 4000", len(got))
	}
	for _, want := range []string{"Page URL: https://www.amazon.co.jp", "MSI GeForce RTX 5090", "SGD 3,488.00",
		"/dp/B01", "ASUS ROG Astral RTX 5090", "SGD 4,100.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("excerpt lacks %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Navigation item 1000", "Footer item 1000", "dib=", "[ref=s1_0]"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("excerpt kept %q", unwanted)
		}
	}
}

func TestCompactKeepsRefsWhereTheModelMayClick(t *testing.T) {
	in := "a\nb\nc\nd\ne\nf\n  - link \"Next page\" [ref=e9] [cursor=pointer]:\n  - generic [ref=e10]: SGD 12.08\n" +
		"  - /url: /dp/X/ref=sr_1?q=1\n  - /url: /dp/X/ref=sr_2?q=2\n  - generic [ref=e11]:\n  - text: \"12\""
	got := compact(in)
	if !strings.Contains(got, `- link "Next page" [ref=e9]:`) {
		t.Errorf("link ref removed: %s", got)
	}
	if strings.Contains(got, "[ref=e10]") || strings.Contains(got, "cursor") {
		t.Errorf("noise kept: %s", got)
	}
	if strings.Count(got, "/url: /dp/X") != 1 {
		t.Errorf("duplicate links not collapsed: %s", got)
	}
	if strings.Contains(got, "[ref=e11]") || strings.Contains(got, `"12"`) {
		t.Errorf("empty container or fragment kept: %s", got)
	}
}

func TestExcerptLeavesShortPagesAlone(t *testing.T) {
	page := "- Page URL: https://x\n- text: hello"
	if excerpt(page, "RTX 5090", 1000) != page {
		t.Fatal("short page was changed")
	}
}

func TestExcerptFallsBackToPageStart(t *testing.T) {
	page := "- Page URL: https://x\n- Page Title: t\n\n\n\n\n" + strings.Repeat("    - text: some content without prices\n", 500)
	got := excerpt(page, "RTX 5090", 2_000)
	if !strings.Contains(got, "No line names the product") || !strings.Contains(got, "some content") || len(got) > 2_100 {
		t.Fatalf("fallback excerpt (%d chars): %.200s", len(got), got)
	}
}
