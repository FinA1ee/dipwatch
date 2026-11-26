// Package ecapi searches stores through 喵有券 (ecapi.cn), which wraps the
// affiliate APIs of JD Union, Taobao Union and Pinduoduo behind one API key.
// Only JD is used so far (api strategy, no LLM tokens). The owner authorizes
// their affiliate accounts in the 喵有券 console and whitelists the server's IP.
package ecapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// DefaultURL is the API host.
const DefaultURL = "https://api.web.ecapi.cn"

const (
	maxBody   = 4 << 20
	jdMaxPage = 30 // the API's page size limit
)

// JD searches JD Union products (jingdong/getJdUnionItems).
type JD struct {
	HTTP   *http.Client
	URL    string // DefaultURL, or a test server
	APIKey string // 喵有券 apkey
	KeyID  string // key_id of the JD Union account authorized in 喵有券
}

func (j *JD) Name() string { return "api" }

func (j *JD) Search(ctx context.Context, q sources.Query) ([]sources.Offer, error) {
	limit := min(max(q.Limit, 1), jdMaxPage)
	params := url.Values{
		"apkey":    {j.APIKey},
		"key_id":   {j.KeyID},
		"keyword":  {q.Product},
		"pageSize": {strconv.Itoa(limit)},
		// Most ordered first: popular new stock before one-off used items.
		"sortName": {"inOrderCount30Days"},
		"sort":     {"desc"},
	}
	body, err := get(ctx, j.HTTP, strings.TrimRight(j.URL, "/")+"/jingdong/getJdUnionItems", params)
	if err != nil {
		return nil, err
	}
	return parseJD(body, limit)
}

// Lookup fetches one JD listing by its itemId: the API takes an item id as the
// keyword. The answer may encode the id differently, so a single result is
// taken as the listing; the offer keeps the id it was stored under.
func (j *JD) Lookup(ctx context.Context, externalID string) (sources.Offer, bool, error) {
	params := url.Values{"apkey": {j.APIKey}, "key_id": {j.KeyID}, "keyword": {externalID}, "pageSize": {"5"}}
	body, err := get(ctx, j.HTTP, strings.TrimRight(j.URL, "/")+"/jingdong/getJdUnionItems", params)
	if err != nil {
		return sources.Offer{}, false, err
	}
	offers, err := parseJD(body, 5)
	if err != nil {
		return sources.Offer{}, false, err
	}
	for _, o := range offers {
		if o.ExternalID == externalID {
			return o, true, nil
		}
	}
	if len(offers) != 1 {
		return sources.Offer{}, false, nil // gone, or ambiguous: do not guess
	}
	o := offers[0]
	o.ExternalID = externalID
	return o, true, nil
}

// get calls the API. The URL carries the API key, so transport errors are
// returned without it: Go's url.Error would otherwise print it into logs.
func get(ctx context.Context, client *http.Client, endpoint string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, errors.New("ecapi: bad request")
	}
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("ecapi: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("ecapi: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ecapi: status %d", resp.StatusCode)
	}
	return body, nil
}

type jdResponse struct {
	Code json.Number     `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type jdItem struct {
	ItemID      string `json:"itemId"`
	SkuName     string `json:"skuName"`
	Owner       string `json:"owner"` // g: JD self-run, p: marketplace seller
	MaterialURL string `json:"materialUrl"`
	PriceInfo   struct {
		Price             json.Number `json:"price"`
		LowestPrice       json.Number `json:"lowestPrice"`
		LowestCouponPrice json.Number `json:"lowestCouponPrice"`
	} `json:"priceInfo"`
	PurchasePriceInfo struct {
		Code          json.Number `json:"code"`
		PurchasePrice json.Number `json:"purchasePrice"` // what one unit costs after coupons and promotions
	} `json:"purchasePriceInfo"`
	ShopInfo struct {
		ShopName string `json:"shopName"`
	} `json:"shopInfo"`
	CategoryInfo struct {
		Cid3Name string `json:"cid3Name"`
	} `json:"categoryInfo"`
}

func parseJD(body []byte, limit int) ([]sources.Offer, error) {
	var r jdResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("ecapi: %w", err)
	}
	if r.Code.String() != "200" {
		return nil, fmt.Errorf("ecapi: jd: code %s: %s", r.Code, r.Msg)
	}
	var data struct {
		List []jdItem `json:"list"`
	}
	if len(r.Data) > 0 && string(r.Data) != "{}" {
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return nil, fmt.Errorf("ecapi: jd data: %w", err)
		}
	}

	offers := make([]sources.Offer, 0, len(data.List))
	for _, it := range data.List {
		purchase := json.Number("")
		if it.PurchasePriceInfo.Code.String() == "200" {
			purchase = it.PurchasePriceInfo.PurchasePrice
		}
		price := firstPositive(purchase, it.PriceInfo.LowestCouponPrice, it.PriceInfo.LowestPrice, it.PriceInfo.Price)
		minor, err := money.ParseMinor(price, "CNY")
		if err != nil || minor <= 0 || it.SkuName == "" || it.ItemID == "" {
			continue
		}
		link := it.MaterialURL
		if link != "" && !strings.HasPrefix(link, "http") {
			link = "https://" + link // the API leaves out the scheme
		}
		if link == "" {
			continue
		}
		seller := "third_party"
		if it.Owner == "g" {
			seller = "official"
		}
		condition := "unknown" // the matcher reads it from the title
		if strings.Contains(it.CategoryInfo.Cid3Name, "二手") {
			condition = "used" // JD's own used-goods categories (拍拍二手)
		}
		raw, _ := json.Marshal(map[string]string{
			"shop": it.ShopInfo.ShopName, "owner": it.Owner, "category": it.CategoryInfo.Cid3Name,
			"list_price": it.PriceInfo.Price.String(), "price_used": price,
		})
		offers = append(offers, sources.Offer{
			ExternalID: it.ItemID, URL: link, Title: it.SkuName, PriceMinor: minor, Currency: "CNY",
			Condition: condition, Availability: "in_stock", SellerType: seller, Raw: raw,
		})
		if len(offers) == limit {
			break
		}
	}
	return offers, nil
}

// firstPositive picks the first price above zero: after coupons, then the
// promotion price, then the list price.
func firstPositive(prices ...json.Number) string {
	for _, p := range prices {
		if v, err := strconv.ParseFloat(p.String(), 64); err == nil && v > 0 {
			return p.String()
		}
	}
	return ""
}
