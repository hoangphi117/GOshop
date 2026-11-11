package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	assistantservice "goshop/services/assistant/internal/service"
	catalogcontract "goshop/services/catalog/contract"
)

// productSearchAdapter maps the catalog HTTP search projection onto the
// assistant's Product. Two shapes exist because the catalog DTO is a
// presentation type and the assistant must not depend on catalog internals.
type productSearchAdapter struct {
	search *catalogcontract.HTTPProductSearchClient
}

func (a productSearchAdapter) Search(ctx context.Context, query string, limit int) ([]assistantservice.Product, error) {
	items, err := a.search.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]assistantservice.Product, 0, len(items))
	for _, p := range items {
		out = append(out, assistantservice.Product{
			UUID:         p.ProductUUID,
			Name:         p.ProductName,
			Slug:         p.ProductSlug,
			Price:        p.ProductPrice,
			Stock:        p.ProductStock,
			CategoryName: p.ProductCategoryName,
			ImageURL:     p.ProductImageURL,
		})
	}
	return out, nil
}

// faqSearchAdapter reads the catalog FAQ route added in the catalog service.
//
// The FAQ knowledge base lives in Postgres behind the catalog service, so this
// reuses GET /api/v1/faqs/search instead of giving the assistant its own DB
// connection and migration set for one query.
type faqSearchAdapter struct {
	baseURL string
	http    *http.Client
}

func newFAQSearchAdapter(baseURL string, timeout time.Duration) (faqSearchAdapter, error) {
	if strings.TrimSpace(baseURL) == "" {
		return faqSearchAdapter{}, fmt.Errorf("faq search: base URL is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return faqSearchAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// faqEnvelope mirrors utils.ResponseSuccess: {"status","data":[...]}.
type faqEnvelope struct {
	Data []struct {
		UUID     string `json:"faq_uuid"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	} `json:"data"`
}

func (a faqSearchAdapter) Search(ctx context.Context, query string, limit int) ([]assistantservice.FAQ, error) {
	if limit <= 0 || limit > 20 {
		limit = 3
	}

	// The FAQ corpus is written in English. Postgres full-text search uses the
	// 'english' dictionary, so a Vietnamese or mixed query ("chính sách hoàn trả
	// đơn hàng") matches nothing even though the answer exists. Retry with the
	// translation hints below before reporting "no FAQs".
	for _, candidate := range faqQueryCandidates(query) {
		items, err := a.fetch(ctx, candidate, limit)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			return items, nil
		}
	}
	return nil, nil
}

func (a faqSearchAdapter) fetch(ctx context.Context, query string, limit int) ([]assistantservice.FAQ, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/api/v1/faqs/search?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("faq search: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("faq search: %w", err)
	}
	defer resp.Body.Close()

	// A non-200 is an error, not "no FAQs": the model must be able to tell an
	// empty knowledge base from a broken one.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("faq search: status %d", resp.StatusCode)
	}

	var env faqEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("faq search: decode: %w", err)
	}

	out := make([]assistantservice.FAQ, 0, len(env.Data))
	for _, f := range env.Data {
		out = append(out, assistantservice.FAQ{UUID: f.UUID, Question: f.Question, Answer: f.Answer})
	}
	return out, nil
}

// faqKeywordHints maps the policy words customers (and the model, copying them)
// use to the English terms the FAQ corpus is written in. It is a translation
// aid, not a classifier: anything not listed simply falls through to the raw
// query, which is what already worked for English input.
//
// ponytail: a fixed table, not an embedded search index. If the catalog ever
// ships a second language, index the FAQs with Meilisearch per locale instead.
var faqKeywordHints = []struct{ words []string; add string }{
	{[]string{"hoàn trả", "hoàn hàng", "trả hàng", "đổi trả", "trả lại", "return", "refund"}, "return refund"},
	{[]string{"giao hàng", "vận chuyển", "ship", "shipping", "delivery"}, "shipping delivery"},
	{[]string{"thanh toán", "payment", "pay"}, "payment"},
	{[]string{"bảo hành", "warranty"}, "warranty"},
	{[]string{"đổi", "exchange"}, "exchange"},
	{[]string{"tài khoản", "account", "mật khẩu", "password"}, "account password"},
	{[]string{"hủy", "huỷ", "cancel"}, "cancel order"},
	{[]string{"khuyến mãi", "giảm giá", "voucher", "coupon", "discount"}, "discount code"},
	{[]string{"địa chỉ", "address"}, "delivery address"},
	{[]string{"theo dõi", "track"}, "track order"},
	{[]string{"liên hệ", "contact", "nhân viên", "human", "agent"}, "contact human"},
}

// faqQueryCandidates returns the queries to try, best first: the raw text, then
// a version with the English equivalents of any recognised policy words appended
// ("chính sách hoàn trả đơn hàng" → "chính sách hoàn trả đơn hàng return refund").
//
// Appending rather than replacing keeps an English query intact: for "return
// policy" the second candidate is a superset that still matches.
func faqQueryCandidates(query string) []string {
	lower := strings.ToLower(query)
	extra := make([]string, 0, 4)
	for _, hint := range faqKeywordHints {
		for _, w := range hint.words {
			if strings.Contains(lower, w) {
				extra = append(extra, hint.add)
				break
			}
		}
	}

	candidates := []string{query}
	if len(extra) > 0 {
		candidates = append(candidates, strings.TrimSpace(query+" "+strings.Join(extra, " ")))
	}
	return candidates
}

// productReaderAdapter is the exact-lookup port over catalog gRPC.
type productReaderAdapter struct {
	client catalogcontract.CatalogClient
}

func (a productReaderAdapter) Get(ctx context.Context, uuid string) (assistantservice.Product, error) {
	p, err := a.client.GetProductByUUID(ctx, uuid)
	if err != nil {
		return assistantservice.Product{}, err
	}
	return assistantservice.Product{
		UUID:         p.ProductUUID,
		Name:         p.ProductName,
		Slug:         p.ProductSlug,
		Price:        p.ProductPrice,
		Stock:        p.ProductStock,
		CategoryName: p.ProductCategoryName,
		ImageURL:     p.ProductImageURL,
	}, nil
}
