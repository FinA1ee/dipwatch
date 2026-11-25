package money

import "testing"

func TestParseMinor(t *testing.T) {
	tests := []struct {
		value, currency string
		want            int64
	}{
		{"1999.99", "USD", 199999},
		{"1999.9", "USD", 199990},
		{"1999", "USD", 199900},
		{"0.05", "CNY", 5},
		{"1,999.00", "USD", 199900},
		{"298000", "JPY", 298000},
		{"298,000", "JPY", 298000},
	}
	for _, tt := range tests {
		got, err := ParseMinor(tt.value, tt.currency)
		if err != nil {
			t.Fatalf("ParseMinor(%q, %s): %v", tt.value, tt.currency, err)
		}
		if got != tt.want {
			t.Errorf("ParseMinor(%q, %s) = %d, want %d", tt.value, tt.currency, got, tt.want)
		}
	}
}

func TestParseMinorRejectsBadInput(t *testing.T) {
	for _, tt := range []struct{ value, currency string }{
		{"", "USD"},
		{"abc", "USD"},
		{"-1.00", "USD"},
		{"1.999", "USD"}, // more decimals than the currency has
		{"100.5", "JPY"}, // yen has no minor unit
		{"1.00", "usd"},  // currency must be an uppercase ISO code
		{"1.00", "US"},
	} {
		if _, err := ParseMinor(tt.value, tt.currency); err == nil {
			t.Errorf("ParseMinor(%q, %q): want error", tt.value, tt.currency)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, c := range []struct {
		minor    int64
		currency string
		want     string
	}{{146990, "CNY", "1469.90 CNY"}, {51871, "JPY", "51871 JPY"}, {5, "USD", "0.05 USD"}} {
		if got := Format(c.minor, c.currency); got != c.want {
			t.Errorf("Format(%d, %s) = %q, want %q", c.minor, c.currency, got, c.want)
		}
	}
}
