package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// submittedOffer is one listing as the model reports it through submit_offers.
type submittedOffer struct {
	Title     string `json:"title"`
	Price     string `json:"price"`
	Currency  string `json:"currency"`
	URL       string `json:"url"`
	Condition string `json:"condition"`
	InStock   *bool  `json:"in_stock"`
	Seller    string `json:"seller"`
}

type submission struct {
	Offers []submittedOffer `json:"offers"`
}

// submitOffersSchema is the JSON Schema of the submit_offers tool.
var submitOffersSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"offers": map[string]any{
			"type":        "array",
			"description": "Listings for the requested product, as shown on the page.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":     map[string]any{"type": "string", "description": "Listing title exactly as shown"},
					"price":     map[string]any{"type": "string", "description": "Price as shown, e.g. \"1,999.99\" or \"¥12999\""},
					"currency":  map[string]any{"type": "string", "description": "ISO 4217 code, e.g. USD, CNY, JPY"},
					"url":       map[string]any{"type": "string", "description": "Link to the listing (absolute or relative)"},
					"condition": map[string]any{"type": "string", "enum": []string{"new", "used", "refurbished", "unknown"}},
					"in_stock":  map[string]any{"type": "boolean"},
					"seller":    map[string]any{"type": "string", "description": "Seller or shop name if shown"},
				},
				"required": []string{"title", "price", "url"},
			},
		},
	},
	"required": []string{"offers"},
}

var (
	nonPrice    = regexp.MustCompile(`[^\d.,]`)
	isoCurrency = regexp.MustCompile(`^[A-Z]{3}$`)
)

// validateSubmission turns the model's submit_offers arguments into offers. Listings
// that fail a check are dropped and described in problems, so the model can be told.
func validateSubmission(arguments string, task Task) (offers []sources.Offer, problems []string) {
	var sub submission
	if err := json.Unmarshal([]byte(arguments), &sub); err != nil {
		return nil, []string{"arguments are not valid JSON: " + err.Error()}
	}
	base, _ := url.Parse(task.StartURL)
	seen := map[string]bool{}
	for i, s := range sub.Offers {
		o, err := validateOffer(s, task, base)
		if err != nil {
			problems = append(problems, fmt.Sprintf("offer %d: %v", i+1, err))
			continue
		}
		if seen[o.ExternalID] {
			continue
		}
		seen[o.ExternalID] = true
		offers = append(offers, o)
		if task.MaxOffers > 0 && len(offers) >= task.MaxOffers {
			break
		}
	}
	if len(sub.Offers) == 0 {
		problems = append(problems, "offers is empty")
	}
	return offers, problems
}

func validateOffer(s submittedOffer, task Task, base *url.URL) (sources.Offer, error) {
	title := strings.Join(strings.Fields(s.Title), " ")
	if title == "" {
		return sources.Offer{}, fmt.Errorf("title is empty")
	}
	// Stores show prices in the visitor's local currency (Amazon shows SGD to a
	// visitor in Singapore), so the currency on the page is the one recorded.
	currency := strings.ToUpper(strings.TrimSpace(s.Currency))
	if currency == "" || currency == "RMB" {
		currency = map[string]string{"": task.Currency, "RMB": "CNY"}[currency]
	}
	if !isoCurrency.MatchString(currency) {
		return sources.Offer{}, fmt.Errorf("currency %q is not an ISO 4217 code such as USD", s.Currency)
	}
	digits := strings.Trim(nonPrice.ReplaceAllString(s.Price, ""), ".,")
	price, err := money.ParseMinor(digits, currency)
	if err != nil || price <= 0 {
		return sources.Offer{}, fmt.Errorf("price %q is not a positive amount", s.Price)
	}

	u, err := url.Parse(strings.TrimSpace(s.URL))
	if err != nil || s.URL == "" {
		return sources.Offer{}, fmt.Errorf("url %q is not valid", s.URL)
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if (u.Scheme != "https" && u.Scheme != "http") || !task.hostAllowed(u.Hostname()) {
		return sources.Offer{}, fmt.Errorf("url %q is not on %s", s.URL, task.StoreName)
	}

	o := sources.Offer{
		ExternalID:   listingID(u),
		URL:          u.String(),
		Title:        title,
		PriceMinor:   price,
		Currency:     currency,
		Condition:    oneOf(s.Condition, "new", "used", "refurbished"),
		Availability: "unknown",
		SellerType:   "unknown",
	}
	if s.InStock != nil {
		o.Availability = map[bool]string{true: "in_stock", false: "out_of_stock"}[*s.InStock]
	}
	raw, _ := json.Marshal(s)
	o.Raw = raw
	return o, nil
}

// listingID identifies a listing by host and path. Query strings are left out
// because stores add tracking parameters that change on every visit.
func listingID(u *url.URL) string {
	sum := sha256.Sum256([]byte(strings.ToLower(u.Hostname()) + strings.TrimRight(u.EscapedPath(), "/")))
	return "agent-" + hex.EncodeToString(sum[:10])
}

func oneOf(v string, allowed ...string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return "unknown"
}

// isEmptySubmission reports whether submit_offers was called with an empty list.
func isEmptySubmission(arguments string) bool {
	var sub submission
	return json.Unmarshal([]byte(arguments), &sub) == nil && len(sub.Offers) == 0
}
