package eval

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

// ReplayBrowser serves one saved page in place of a live browser. It answers the
// same tools the agent uses, so the agent loop runs unchanged.
type ReplayBrowser struct {
	Page  string
	Calls []string
}

const maxFindDefault = 15

func (b *ReplayBrowser) Tools(context.Context) ([]llm.Tool, error) {
	obj := func(props map[string]any, required ...string) map[string]any {
		s := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	str := map[string]any{"type": "string"}
	num := map[string]any{"type": "number"}
	return []llm.Tool{
		{Name: "browser_navigate", Description: "Navigate to a URL", Parameters: obj(map[string]any{"url": str}, "url")},
		{Name: "browser_snapshot", Description: "Capture accessibility snapshot of the current page", Parameters: obj(map[string]any{})},
		{Name: "browser_find", Description: "Search the page snapshot for text or a regular expression",
			Parameters: obj(map[string]any{"text": str, "regex": str, "maxResults": num})},
		{Name: "browser_click", Description: "Click an element", Parameters: obj(map[string]any{"element": str, "target": str}, "target")},
		{Name: "browser_type", Description: "Type text into an element", Parameters: obj(map[string]any{"target": str, "text": str}, "target", "text")},
		{Name: "browser_press_key", Description: "Press a key", Parameters: obj(map[string]any{"key": str}, "key")},
		{Name: "browser_wait_for", Description: "Wait for text or time", Parameters: obj(map[string]any{"text": str, "time": num})},
	}, nil
}

func (b *ReplayBrowser) Call(_ context.Context, name string, args map[string]any) (string, error) {
	b.Calls = append(b.Calls, name)
	switch name {
	case "browser_navigate":
		u, _ := args["url"].(string)
		return fmt.Sprintf("### Ran Playwright code\n```js\nawait page.goto(%q);\n```\n%s", u, pageHeader(b.Page)), nil
	case "browser_snapshot":
		return b.Page, nil
	case "browser_find":
		return find(b.Page, args)
	case "browser_click", "browser_type", "browser_press_key", "browser_wait_for":
		// A saved page cannot change; the harness re-snapshots and gets the same page.
		return "### Result\nDone. (Replay: the page did not change.)\n" + pageHeader(b.Page), nil
	default:
		return "", fmt.Errorf("replay browser: tool %s not supported", name)
	}
}

func (b *ReplayBrowser) Close() error { return nil }

func pageHeader(page string) string {
	var out []string
	for _, l := range strings.Split(page, "\n") {
		if strings.HasPrefix(l, "- Page URL:") || strings.HasPrefix(l, "- Page Title:") {
			out = append(out, l)
		}
		if len(out) == 2 {
			break
		}
	}
	return "### Page\n" + strings.Join(out, "\n")
}

// find mimics Playwright MCP's browser_find: matching lines with a little context.
func find(page string, args map[string]any) (string, error) {
	limit := maxFindDefault
	if n, ok := args["maxResults"].(float64); ok && n > 0 {
		limit = int(n)
	}
	var match func(string) bool
	var label string
	if re, _ := args["regex"].(string); re != "" {
		label = "/" + re + "/"
		re = strings.TrimSuffix(strings.TrimPrefix(re, "/"), "/")
		r, err := regexp.Compile(re)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %w", err)
		}
		match = r.MatchString
	} else if t, _ := args["text"].(string); t != "" {
		label = strconvQuote(t)
		lower := strings.ToLower(t)
		match = func(l string) bool { return strings.Contains(strings.ToLower(l), lower) }
	} else {
		return "", errors.New("provide text or regex")
	}

	lines := strings.Split(page, "\n")
	var hits []int
	for i, l := range lines {
		if match(l) {
			hits = append(hits, i)
		}
	}
	var b strings.Builder
	if len(hits) == 0 {
		fmt.Fprintf(&b, "### Result\nNo matches found for %s.\n", label)
		return b.String(), nil
	}
	fmt.Fprintf(&b, "### Result\nFound %d matches for %s", len(hits), label)
	if len(hits) > limit {
		fmt.Fprintf(&b, " (showing first %d)", limit)
	}
	b.WriteString(":\n")
	for _, i := range hits[:min(limit, len(hits))] {
		for j := max(0, i-2); j <= min(len(lines)-1, i+2); j++ {
			b.WriteString(lines[j])
			b.WriteByte('\n')
		}
		b.WriteString("---\n")
	}
	return b.String(), nil
}

func strconvQuote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
