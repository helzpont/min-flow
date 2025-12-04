// Package main demonstrates building a web scraping/API aggregation pipeline
// that fetches data from multiple sources, transforms it, and combines results.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/combine"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
	flowhttp "github.com/lguimbarda/min-flow/flow/http"
	flowjson "github.com/lguimbarda/min-flow/flow/json"
	"github.com/lguimbarda/min-flow/flow/parallel"
)

// Product represents a product from an API
type Product struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
	InStock  bool    `json:"in_stock"`
}

func main() {
	fmt.Println("=== Web Data Pipeline Example ===")
	fmt.Println()

	// Create mock API servers
	server1, server2 := createMockServers()
	defer server1.Close()
	defer server2.Close()

	multiSourceAggregation(server1.URL, server2.URL)
	parallelFetching()
	streamingAPI()
}

// multiSourceAggregation fetches and combines data from multiple APIs
func multiSourceAggregation(url1, url2 string) {
	fmt.Println("--- Multi-Source Aggregation ---")
	ctx := context.Background()

	// Fetch from first source
	source1 := flowhttp.Get(url1 + "/products")

	// Fetch from second source
	source2 := flowhttp.Get(url2 + "/products")

	// Parse responses
	products1 := flow.FlatMap(func(resp flowhttp.Response) ([]Product, error) {
		var products []Product
		if err := json.Unmarshal(resp.Body, &products); err != nil {
			return nil, err
		}
		return products, nil
	}).Apply(ctx, source1)

	products2 := flow.FlatMap(func(resp flowhttp.Response) ([]Product, error) {
		var products []Product
		if err := json.Unmarshal(resp.Body, &products); err != nil {
			return nil, err
		}
		return products, nil
	}).Apply(ctx, source2)

	// Merge products from both sources
	allProducts := combine.Merge(products1, products2)

	// Filter to in-stock items only
	inStock := filter.Where(func(p Product) bool {
		return p.InStock
	}).Apply(ctx, allProducts)

	// Filter by price range
	affordable := filter.Where(func(p Product) bool {
		return p.Price <= 100
	}).Apply(ctx, inStock)

	fmt.Println("In-stock products under $100:")
	for res := range affordable.All(ctx) {
		if res.IsValue() {
			p := res.Value()
			fmt.Printf("  %s ($%.2f) - %s\n", p.Name, p.Price, p.Category)
		}
	}
	fmt.Println()
}

// parallelFetching demonstrates concurrent API calls
func parallelFetching() {
	fmt.Println("--- Parallel URL Fetching ---")

	// Create multiple mock endpoints
	endpoints := make([]*httptest.Server, 5)
	for i := 0; i < 5; i++ {
		delay := time.Duration(i*50) * time.Millisecond
		endpoints[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay)
			json.NewEncoder(w).Encode(map[string]any{
				"endpoint": r.URL.Path,
				"status":   "ok",
			})
		}))
		defer endpoints[i].Close()
	}

	ctx := context.Background()

	// Create stream of URLs
	urls := flow.FromSlice([]string{
		endpoints[0].URL + "/api/1",
		endpoints[1].URL + "/api/2",
		endpoints[2].URL + "/api/3",
		endpoints[3].URL + "/api/4",
		endpoints[4].URL + "/api/5",
	})

	// Fetch in parallel with 3 workers
	start := time.Now()
	responses := parallel.Map(3, func(url string) flowhttp.Response {
		resp, err := http.Get(url)
		if err != nil {
			return flowhttp.Response{}
		}
		defer resp.Body.Close()
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		return flowhttp.Response{
			StatusCode: resp.StatusCode,
			Body:       body[:n],
		}
	}).Apply(ctx, urls)

	var count int
	for res := range responses.All(ctx) {
		if res.IsValue() {
			count++
		}
	}
	fmt.Printf("Fetched %d endpoints in %v (parallel)\n\n", count, time.Since(start))
}

// streamingAPI shows processing a streaming API response
func streamingAPI() {
	fmt.Println("--- Streaming API Processing ---")

	// Mock a streaming API that emits JSON lines
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		products := []Product{
			{ID: 1, Name: "Widget A", Price: 19.99, Category: "Tools", InStock: true},
			{ID: 2, Name: "Gadget B", Price: 29.99, Category: "Electronics", InStock: true},
			{ID: 3, Name: "Thing C", Price: 9.99, Category: "Tools", InStock: false},
		}
		for _, p := range products {
			data, _ := json.Marshal(p)
			fmt.Fprintf(w, "%s\n", data)
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx := context.Background()

	// Stream lines from the API
	lines := flowhttp.GetLines(server.URL)

	// Parse each JSON line into a Product
	products := flowjson.Decode[Product]().Apply(ctx, lines)

	fmt.Println("Streaming products:")
	for res := range products.All(ctx) {
		if res.IsValue() {
			p := res.Value()
			status := "✓"
			if !p.InStock {
				status = "✗"
			}
			fmt.Printf("  [%s] %s - $%.2f\n", status, p.Name, p.Price)
		}
	}
	fmt.Println()

	retryFetchExample()
}

// retryFetchExample shows resilient fetching with retries
func retryFetchExample() {
	fmt.Println("--- Resilient Fetching with Retry ---")

	// Mock a flaky server
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 3 {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	ctx := context.Background()

	// Wrap fetch with retry logic
	urls := flow.FromSlice([]string{server.URL})

	fetcher := func(url string) (string, error) {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		return result["status"], nil
	}

	// Retry up to 5 times with backoff
	backoff := flowerrors.ExponentialBackoff(50*time.Millisecond, 1*time.Second)
	results := flowerrors.RetryWithBackoff(5, backoff, fetcher).Apply(ctx, urls)

	for res := range results.All(ctx) {
		if res.IsError() {
			fmt.Printf("  Failed after retries: %v\n", res.Error())
		} else {
			fmt.Printf("  Success after %d attempts: %s\n", requestCount, res.Value())
		}
	}
	fmt.Println()
}

// createMockServers creates two mock product API servers
func createMockServers() (*httptest.Server, *httptest.Server) {
	products1 := []Product{
		{ID: 1, Name: "Laptop", Price: 999.99, Category: "Electronics", InStock: true},
		{ID: 2, Name: "Mouse", Price: 29.99, Category: "Electronics", InStock: true},
		{ID: 3, Name: "Keyboard", Price: 79.99, Category: "Electronics", InStock: false},
	}

	products2 := []Product{
		{ID: 4, Name: "Desk Lamp", Price: 45.00, Category: "Home", InStock: true},
		{ID: 5, Name: "Chair", Price: 199.00, Category: "Furniture", InStock: true},
		{ID: 6, Name: "Monitor Stand", Price: 35.00, Category: "Accessories", InStock: true},
	}

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(products1)
	}))

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(products2)
	}))

	return server1, server2
}
