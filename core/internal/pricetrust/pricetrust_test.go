package pricetrust

import "testing"

func TestUnusual(t *testing.T) {
	prices := []Price{
		{CNY: 1_469_900, Known: true, Variant: "24GB"},
		{CNY: 1_479_900, Known: true, Variant: "24GB"},
		{CNY: 1_685_900, Known: true, Variant: "24GB"},
		{CNY: 9_589_408, Known: true, Variant: "24GB"}, // reseller listing: unusual
		{CNY: 3_099_900, Known: true, Variant: "48GB"}, // its own variant, too few to judge
		{CNY: 3_084_400, Known: true, Variant: "48GB"},
		{CNY: 500, Known: false, Variant: "24GB"}, // no rate: never judged
	}
	got := Unusual(prices)
	want := []bool{false, false, false, true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("price %d (%d): unusual %v, want %v", i, prices[i].CNY, got[i], want[i])
		}
	}
	// Far too cheap is unusual too, e.g. a deposit or a part.
	if got := Unusual([]Price{{CNY: 100, Known: true}, {CNY: 1000, Known: true}, {CNY: 1100, Known: true}}); !got[0] {
		t.Error("a price under half the median was not unusual")
	}
}
