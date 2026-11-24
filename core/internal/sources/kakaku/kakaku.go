// Package kakaku reads search results from Kakaku.com, a Japanese price comparison
// site whose search pages are rendered on the server (html strategy, no tokens).
package kakaku

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"

	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

const (
	DefaultBaseURL = "https://search.kakaku.com"
	userAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36"
	maxBody        = 4 << 20
)

var (
	catalogItemPath = regexp.MustCompile(`^/item/(K\d+)/?$`)
	priceDigits     = regexp.MustCompile(`[\d,]+`)
)

type Kakaku struct {
	client  *http.Client
	baseURL string
}

func New(client *http.Client, baseURL string) *Kakaku {
	return &Kakaku{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (k *Kakaku) Name() string { return "html" }

func (k *Kakaku) Search(ctx context.Context, q sources.Query) ([]sources.Offer, error) {
	target := fmt.Sprintf("%s/%s/", k.baseURL, url.PathEscape(q.Product))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "ja,en;q=0.8")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kakaku search: HTTP %d", resp.StatusCode)
	}

	// The site serves Shift_JIS; decode based on the Content-Type header and the page's meta tag.
	body, err := charset.NewReader(io.LimitReader(resp.Body, maxBody), resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("kakaku search: decode: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("kakaku search: parse: %w", err)
	}
	return parse(doc, q.Limit), nil
}

func parse(doc *goquery.Document, limit int) []sources.Offer {
	var offers []sources.Offer
	doc.Find("div.p-item").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		if o, ok := parseItem(item); ok {
			offers = append(offers, o)
		}
		return limit <= 0 || len(offers) < limit
	})
	return offers
}

func parseItem(item *goquery.Selection) (sources.Offer, bool) {
	link := item.Find(".p-item_name a").First()
	title := clean(link.Text())
	href, _ := link.Attr("href")
	price, ok := parsePrice(item.Find(".p-item_priceNum").First().Text())
	if title == "" || href == "" || !ok {
		return sources.Offer{}, false
	}
	if maker := clean(item.Find(".p-item_maker").First().Text()); maker != "" && !strings.HasPrefix(title, maker) {
		title = maker + " " + title
	}

	o := sources.Offer{
		Title:        title,
		PriceMinor:   price,
		Currency:     "JPY",
		Condition:    "new", // Kakaku lists new products; used goods are a separate site section.
		Availability: "in_stock",
		SellerType:   "unknown",
	}

	u, err := url.Parse(href)
	if m := catalogItemPath.FindStringSubmatch(u.Path); err == nil && m != nil {
		// A catalog page that compares several shops; the price is the lowest one.
		o.ExternalID = m[1]
		o.URL = "https://kakaku.com/item/" + m[1] + "/"
	} else {
		// A single listing from an online mall, linked through a tracking redirect.
		sum := sha256.Sum256([]byte(href))
		o.ExternalID = "mall-" + hex.EncodeToString(sum[:8])
		o.URL = href
		o.SellerType = "third_party"
	}
	return o, true
}

// parsePrice reads "1,280,000～" (the ～ marks a lowest price) into yen.
func parsePrice(s string) (int64, bool) {
	digits := priceDigits.FindString(s)
	if digits == "" {
		return 0, false
	}
	n, err := money.ParseMinor(digits, "JPY")
	return n, err == nil && n > 0
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }
