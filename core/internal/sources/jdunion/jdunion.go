// Package jdunion searches JD through its affiliate API, JD Union
// (京东联盟, jd.union.open.goods.query). JD's own search page shows a login
// wall to servers, so this is the only way in. It needs an app key and secret
// from the owner's JD Union account (api strategy, no LLM tokens).
package jdunion

import (
	"context"
	"crypto/md5" //nolint:gosec // the JD router's signature scheme, not a security choice of ours
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/money"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// DefaultURL is the JD open platform router.
const DefaultURL = "https://api.jd.com/routerjson"

const (
	method  = "jd.union.open.goods.query"
	maxBody = 4 << 20
	maxPage = 30 // the API's page size limit
)

// China is the time zone the router expects timestamps in.
var China = time.FixedZone("CST", 8*60*60)

type Client struct {
	HTTP      *http.Client
	URL       string
	AppKey    string
	AppSecret string
	Now       func() time.Time // nil: time.Now
}

func (c *Client) Name() string { return "api" }

func (c *Client) Search(ctx context.Context, q sources.Query) ([]sources.Offer, error) {
	limit := min(max(q.Limit, 1), maxPage)
	payload, err := json.Marshal(map[string]any{
		"goodsReqDTO": map[string]any{"keyword": q.Product, "pageIndex": 1, "pageSize": limit},
	})
	if err != nil {
		return nil, err
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	params := map[string]string{
		"method":            method,
		"app_key":           c.AppKey,
		"format":            "json",
		"v":                 "1.0",
		"sign_method":       "md5",
		"timestamp":         now().In(China).Format("2006-01-02 15:04:05"),
		"360buy_param_json": string(payload),
	}
	params["sign"] = Sign(c.AppSecret, params)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jd union: status %d", resp.StatusCode)
	}
	return parse(body, limit)
}

// Sign is the router's signature: the secret, then every parameter as key and
// value sorted by key, then the secret again; MD5, upper-case hex.
func Sign(secret string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "sign" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(secret)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String())) //nolint:gosec // required by the JD router
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// The router wraps the API's own JSON as a string inside its envelope.
type envelope struct {
	Response *struct {
		Code   string `json:"code"`
		Result string `json:"queryResult"`
	} `json:"jd_union_open_goods_query_responce"` //nolint:misspell // JD's own spelling of the key
	ErrorResponse *struct {
		Code    string `json:"code"`
		Message string `json:"zh_desc"`
		English string `json:"en_desc"`
	} `json:"error_response"`
}

type result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		SkuID     json.Number `json:"skuId"`
		SkuName   string      `json:"skuName"`
		Owner     string      `json:"owner"` // g: JD self-run, p: marketplace seller
		URL       string      `json:"materialUrl"`
		PriceInfo struct {
			Price             json.Number `json:"price"`
			LowestPrice       json.Number `json:"lowestPrice"`
			LowestCouponPrice json.Number `json:"lowestCouponPrice"`
		} `json:"priceInfo"`
		ShopInfo struct {
			ShopName string `json:"shopName"`
		} `json:"shopInfo"`
	} `json:"data"`
}

func parse(body []byte, limit int) ([]sources.Offer, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("jd union: %w", err)
	}
	if e := env.ErrorResponse; e != nil {
		return nil, fmt.Errorf("jd union: error %s: %s %s", e.Code, e.Message, e.English)
	}
	if env.Response == nil || env.Response.Result == "" {
		return nil, errors.New("jd union: empty response")
	}
	var r result
	if err := json.Unmarshal([]byte(env.Response.Result), &r); err != nil {
		return nil, fmt.Errorf("jd union: result: %w", err)
	}
	if r.Code != 200 {
		return nil, fmt.Errorf("jd union: code %d: %s", r.Code, r.Message)
	}

	offers := make([]sources.Offer, 0, len(r.Data))
	for _, g := range r.Data {
		price := firstPositive(g.PriceInfo.LowestCouponPrice, g.PriceInfo.LowestPrice, g.PriceInfo.Price)
		minor, err := money.ParseMinor(price, "CNY")
		if err != nil || minor <= 0 || g.SkuName == "" {
			continue
		}
		link := g.URL
		if link == "" || !strings.HasPrefix(link, "http") {
			link = "https://item.jd.com/" + g.SkuID.String() + ".html"
		}
		seller := "third_party"
		if g.Owner == "g" {
			seller = "official"
		}
		raw, _ := json.Marshal(map[string]string{"shop": g.ShopInfo.ShopName, "owner": g.Owner})
		offers = append(offers, sources.Offer{
			ExternalID: g.SkuID.String(), URL: link, Title: g.SkuName, PriceMinor: minor, Currency: "CNY",
			Condition: "new", Availability: "in_stock", SellerType: seller, Raw: raw,
		})
		if len(offers) == limit {
			break
		}
	}
	return offers, nil
}

// firstPositive picks the price a shopper pays: after coupons, then the
// promotion price, then the list price.
func firstPositive(prices ...json.Number) string {
	for _, p := range prices {
		if v, err := strconv.ParseFloat(p.String(), 64); err == nil && v > 0 {
			return p.String()
		}
	}
	return ""
}
