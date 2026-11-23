// Command sources checks, once, whether each candidate store's search page for a
// product can be fetched with plain HTTP, and what the response looks like.
//
// It sends one GET per source, sequentially, with a pause between them. It does
// not retry, follow pagination, or try to get past blocks: a block is a result.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type source struct {
	ID, Name, Region, URL string
}

const query = "RTX 5090"

var sources = []source{
	{"jd", "JD.com 京东", "CN", "https://search.jd.com/Search?keyword=RTX%205090&enc=utf-8"},
	{"taobao", "Taobao 淘宝", "CN", "https://s.taobao.com/search?q=RTX%205090"},
	{"xianyu", "Xianyu 闲鱼", "CN", "https://www.goofish.com/search?q=RTX%205090"},
	{"pdd", "Pinduoduo 拼多多", "CN", "https://mobile.yangkeduo.com/search_result.html?search_key=RTX5090"},
	{"smzdm", "SMZDM 什么值得买", "CN", "https://search.smzdm.com/?c=home&s=RTX%205090"},
	{"ebay-us", "eBay", "US", "https://www.ebay.com/sch/i.html?_nkw=rtx+5090"},
	{"newegg", "Newegg", "US", "https://www.newegg.com/p/pl?d=rtx+5090"},
	{"amazon-us", "Amazon US", "US", "https://www.amazon.com/s?k=rtx+5090"},
	{"bestbuy", "Best Buy", "US", "https://www.bestbuy.com/site/searchpage.jsp?st=rtx+5090"},
	{"bhphoto", "B&H Photo", "US", "https://www.bhphotovideo.com/c/search?q=rtx%205090"},
	{"pcpartpicker", "PCPartPicker", "US", "https://pcpartpicker.com/search/?q=rtx+5090"},
	{"amazon-jp", "Amazon Japan", "JP", "https://www.amazon.co.jp/s?k=rtx+5090"},
	{"kakaku", "Kakaku.com 価格.com", "JP", "https://kakaku.com/search_results/rtx%205090/"},
}

var (
	pricePattern = regexp.MustCompile(`(?:¥|￥|\$|US \$|円)\s?\d[\d,]{2,}|\d[\d,]{2,}\s?円`)
	blockWords   = []string{"captcha", "验证", "robot check", "are you a human", "access denied", "滑块", "unusual traffic", "px-captcha", "cf-chl"}
	loginWords   = []string{"passport.jd.com", "login.taobao.com", "/login", "登录"}
)

type result struct {
	src      source
	status   int
	finalURL string
	bytes    int
	prices   int
	mentions int
	jsonLD   bool
	block    string
	login    bool
	err      error
	elapsed  time.Duration
}

func main() {
	client := &http.Client{Timeout: 20 * time.Second}
	results := make([]result, 0, len(sources))
	for i, s := range sources {
		if i > 0 {
			time.Sleep(2 * time.Second)
		}
		r := check(client, s)
		results = append(results, r)
		fmt.Fprintf(os.Stderr, "%-13s %3d %7d bytes %3d prices\n", s.ID, r.status, r.bytes, r.prices)
	}
	printTable(results)
}

// r is a named result so the deferred timer writes to the value that is returned.
func check(client *http.Client, s source) (r result) {
	r.src = s
	start := time.Now()
	defer func() { r.elapsed = time.Since(start) }()

	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	if err != nil {
		r.err = err
		return r
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8,zh-CN;q=0.6,ja;q=0.4")

	resp, err := client.Do(req)
	if err != nil {
		r.err = err
		return r
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		r.err = err
		return r
	}
	html := string(body)
	lower := strings.ToLower(html)

	r.status = resp.StatusCode
	r.finalURL = resp.Request.URL.String()
	r.bytes = len(body)
	r.prices = len(pricePattern.FindAllString(html, -1))
	r.mentions = strings.Count(lower, "5090")
	r.jsonLD = strings.Contains(lower, "application/ld+json")
	for _, w := range blockWords {
		if strings.Contains(lower, w) {
			r.block = w
			break
		}
	}
	for _, w := range loginWords {
		if strings.Contains(strings.ToLower(r.finalURL), w) {
			r.login = true
			break
		}
	}
	return r
}

func printTable(results []result) {
	fmt.Printf("Query: %q, fetched %s\n\n", query, time.Now().Format("2006-01-02 15:04 MST"))
	fmt.Println("| Source | Region | HTTP | Size | Price matches | \"5090\" mentions | JSON-LD | Block signal | Login redirect | Time |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|---|")
	for _, r := range results {
		if r.err != nil {
			fmt.Printf("| %s | %s | error | | | | | %s | | %s |\n", r.src.Name, r.src.Region, shortErr(r.err), r.elapsed.Round(100*time.Millisecond))
			continue
		}
		fmt.Printf("| %s | %s | %d | %s | %d | %d | %s | %s | %s | %s |\n",
			r.src.Name, r.src.Region, r.status, kb(r.bytes), r.prices, r.mentions,
			yes(r.jsonLD), dash(r.block), yes(r.login), r.elapsed.Round(100*time.Millisecond))
	}
}

func kb(n int) string { return fmt.Sprintf("%d KB", n/1024) }

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return "`" + s + "`"
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 60 {
		s = s[:60] + "..."
	}
	return strings.ReplaceAll(s, "|", "/")
}
