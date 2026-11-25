// Package money converts between decimal price strings and integer minor units.
// Prices are stored as minor units (cents; yen for JPY) to avoid floating point.
package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// zeroDecimal lists ISO 4217 currencies without a minor unit that sources may return.
var zeroDecimal = map[string]bool{"JPY": true, "KRW": true}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

// Exponent returns the number of decimal places of a currency's minor unit.
func Exponent(currency string) int {
	if zeroDecimal[currency] {
		return 0
	}
	return 2
}

// ParseMinor parses a non-negative decimal amount such as "1,999.99" into minor units.
func ParseMinor(value, currency string) (int64, error) {
	if !currencyCode.MatchString(currency) {
		return 0, fmt.Errorf("invalid currency code %q", currency)
	}
	v := strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	if v == "" || strings.HasPrefix(v, "-") || strings.HasPrefix(v, "+") {
		return 0, fmt.Errorf("invalid amount %q", value)
	}

	whole, frac, _ := strings.Cut(v, ".")
	exp := Exponent(currency)
	if len(frac) > exp {
		return 0, fmt.Errorf("amount %q has more than %d decimals for %s", value, exp, currency)
	}
	frac += strings.Repeat("0", exp-len(frac))

	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", value, err)
	}
	return n, nil
}

// Format writes minor units as "1469.00 CNY" or "51871 JPY", for messages.
func Format(minor int64, currency string) string {
	exp := Exponent(currency)
	if exp == 0 {
		return fmt.Sprintf("%d %s", minor, currency)
	}
	div := int64(1)
	for range exp {
		div *= 10
	}
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%0*d %s", sign, minor/div, exp, minor%div, currency)
}

// Money is an amount in minor units of an ISO 4217 currency, as the API sends it.
type Money struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency" pattern:"^[A-Z]{3}$"`
}
