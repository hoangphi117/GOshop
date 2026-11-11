package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFAQSearchAdapter(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":[
			{"faq_uuid":"f-1","question":"Do you ship to Hanoi?","answer":"2-4 business days."}
		]}`))
	}))
	defer srv.Close()

	a, err := newFAQSearchAdapter(srv.URL+"/", 2*time.Second)
	if err != nil {
		t.Fatalf("newFAQSearchAdapter: %v", err)
	}
	items, err := a.Search(context.Background(), "shipping", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/api/v1/faqs/search" || gotQuery != "shipping" {
		t.Fatalf("path=%q q=%q", gotPath, gotQuery)
	}
	if len(items) != 1 || items[0].UUID != "f-1" || items[0].Answer != "2-4 business days." {
		t.Fatalf("items = %+v", items)
	}
}

// A non-200 must be an error, not an empty knowledge base: the model has to
// tell "nothing matches" from "catalog is down".
func TestFAQSearchAdapterErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a, _ := newFAQSearchAdapter(srv.URL, time.Second)
	if _, err := a.Search(context.Background(), "shipping", 3); err == nil {
		t.Fatal("expected an error on 503")
	}
}

func TestFAQSearchAdapterRequiresBaseURL(t *testing.T) {
	if _, err := newFAQSearchAdapter("   ", time.Second); err == nil {
		t.Fatal("expected an error for an empty base URL")
	}
}

// Regression from a live report: the customer typed "What is your return
// policy?" (and the model forwarded Vietnamese, "chính sách hoàn trả đơn hàng").
// The FAQ corpus is English, so the raw query returned zero rows and the bot
// said the shop had no return policy. The retry with English policy terms must
// find it.
func TestFAQSearchAdapterRetriesWithEnglishTerms(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		seen = append(seen, q)
		w.Header().Set("Content-Type", "application/json")
		// The fake catalog only understands English keywords, like the real one.
		if strings.Contains(q, "return") {
			_, _ = w.Write([]byte(`{"status":"success","data":[
				{"faq_uuid":"f-1","question":"How do I return an item?","answer":"Within 7 days."}
			]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer srv.Close()

	a, _ := newFAQSearchAdapter(srv.URL, 2*time.Second)
	items, err := a.Search(context.Background(), "chính sách hoàn trả đơn hàng", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) != 1 || items[0].UUID != "f-1" {
		t.Fatalf("items = %+v (queries tried: %q)", items, seen)
	}
	if len(seen) < 2 {
		t.Fatalf("expected a retry, queries = %q", seen)
	}
}

func TestFAQQueryCandidates(t *testing.T) {
	cases := map[string]string{
		"chính sách hoàn trả đơn hàng": "chính sách hoàn trả đơn hàng return refund",
		"What is your return policy?":  "What is your return policy? return refund",
		"giao hàng mất bao lâu":        "giao hàng mất bao lâu shipping delivery",
		// No recognised policy word: one candidate, no pointless retry.
		"iphone 16": "iphone 16",
	}
	for in, wantFirst := range cases {
		got := faqQueryCandidates(in)
		if len(got) == 0 || got[len(got)-1] != wantFirst {
			t.Errorf("faqQueryCandidates(%q) = %q, want last %q", in, got, wantFirst)
		}
	}
}
