// Command source-smoke runs one source adapter against the live site and prints what
// it found. It sends a single search request and touches no database.
//
//	go run ./cmd/source-smoke -source kakaku -q "RTX 5090"
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/sources/kakaku"
)

func main() {
	source := flag.String("source", "kakaku", "source id")
	query := flag.String("q", "RTX 5090", "product to search for")
	limit := flag.Int("limit", 50, "maximum offers")
	flag.Parse()

	client := &http.Client{Timeout: 30 * time.Second}
	strategies := map[string]sources.Strategy{
		"kakaku": kakaku.New(client, kakaku.DefaultBaseURL),
	}
	s, ok := strategies[*source]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown source %q\n", *source)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	offers, err := s.Search(ctx, sources.Query{Product: *query, Limit: *limit})
	if err != nil {
		fmt.Fprintln(os.Stderr, "search failed:", err)
		os.Exit(1)
	}
	for _, o := range offers {
		fmt.Printf("%10d %s  %-14s  %s\n", o.PriceMinor, o.Currency, o.ExternalID, o.Title)
	}
	fmt.Printf("\n%d offers from %s (%s strategy)\n", len(offers), *source, s.Name())
}
