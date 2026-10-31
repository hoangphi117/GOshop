package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"goshop/shared/pkg/auth"
)

type CheckoutRequestItem struct {
	ProductUUID string `json:"product_uuid"`
	Quantity    int32  `json:"quantity"`
}

type CheckoutRequestBody struct {
	ShippingAddress string                `json:"shipping_address"`
	ShippingName    string                `json:"shipping_name"`
	ShippingPhone   string                `json:"shipping_phone"`
	ShippingFee     float64               `json:"shipping_fee"`
	PaymentMethod   int32                 `json:"payment_method"`
	Items           []CheckoutRequestItem `json:"items"`
}

func main() {
	ctx := context.Background()
	testRunID := uuid.New().String()[:8]
	fmt.Printf("====================================================================\n")
	fmt.Printf("🚀 STARTING SAGA CHECKOUT CONCURRENCY & OVERSELL LOAD TEST\n")
	fmt.Printf("Test Run ID: %s | Target: http://localhost:8080\n", testRunID)
	fmt.Printf("====================================================================\n\n")

	// 1. Connect to PostgreSQL databases
	invDBUrl := "postgres://postgres:super@password@localhost:5432/shopping_inventory?sslmode=disable"
	orderDBUrl := "postgres://postgres:super@password@localhost:5432/shopping?sslmode=disable"

	invPool, err := pgxpool.New(ctx, invDBUrl)
	if err != nil {
		fmt.Printf("❌ Failed to connect to shopping_inventory: %v\n", err)
		os.Exit(1)
	}
	defer invPool.Close()

	orderPool, err := pgxpool.New(ctx, orderDBUrl)
	if err != nil {
		fmt.Printf("❌ Failed to connect to shopping: %v\n", err)
		os.Exit(1)
	}
	defer orderPool.Close()

	// 2. Select a target product
	var productID int32
	var productUUID string
	var productName string
	err = orderPool.QueryRow(ctx, "SELECT product_id, product_uuid, product_name FROM products WHERE product_status = 1 LIMIT 1").
		Scan(&productID, &productUUID, &productName)
	if err != nil {
		fmt.Printf("❌ Failed to find an active product: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("🎯 Selected Product:\n   ID: %d\n   UUID: %s\n   Name: %s\n\n", productID, productUUID, productName)

	// 3. Save original stock and set stock = 10
	var originalStock int32
	err = invPool.QueryRow(ctx, "SELECT quantity FROM inventory_stocks WHERE product_id = $1", productID).Scan(&originalStock)
	if err != nil {
		// If not found in inventory_stocks, insert initial stock
		originalStock = 100
		_, _ = invPool.Exec(ctx, "INSERT INTO inventory_stocks (product_id, product_uuid, quantity, reserved_quantity) VALUES ($1, $2, $3, 0)",
			productID, productUUID, originalStock)
	}

	const testInitialStock = int32(10)
	const concurrentUsers = 30

	_, err = invPool.Exec(ctx, "UPDATE inventory_stocks SET quantity = $1, reserved_quantity = 0 WHERE product_id = $2",
		testInitialStock, productID)
	if err != nil {
		fmt.Printf("❌ Failed to set stock to %d: %v\n", testInitialStock, err)
		os.Exit(1)
	}
	fmt.Printf("📦 Initial Stock set to: %d units in inventory_stocks\n", testInitialStock)

	// Restore original stock at exit
	defer func() {
		_, _ = invPool.Exec(ctx, "UPDATE inventory_stocks SET quantity = $1, reserved_quantity = 0 WHERE product_id = $2", originalStock, productID)
		fmt.Printf("\n🔄 Restored original stock (%d units) for product %d.\n", originalStock, productID)
	}()

	// 4. Generate JWT for a real test user
	var userID int32
	var userUUID string
	var userEmail string
	err = orderPool.QueryRow(ctx, "SELECT user_id, user_uuid, user_email FROM users WHERE user_status = 1 LIMIT 1").
		Scan(&userID, &userUUID, &userEmail)
	if err != nil {
		fmt.Printf("❌ Failed to find an active user: %v\n", err)
		os.Exit(1)
	}

	jwtService := auth.NewJWTService(nil)
	token, err := jwtService.GenerateAccessToken(auth.UserTokenPayload{
		UserID:   userID,
		UserUUID: userUUID,
		Email:    userEmail,
		Role:     1,
	})
	if err != nil {
		fmt.Printf("❌ Failed to generate JWT token: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("👤 Authenticated as: %s (ID: %d, UUID: %s)\n", userEmail, userID, userUUID)

	// 5. Prepare concurrent requests
	fmt.Printf("\n⚡ Firing %d concurrent checkout requests (1 unit each) simultaneously...\n", concurrentUsers)

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	reqBody := CheckoutRequestBody{
		ShippingAddress: "123 Concurrency Test Blvd, High Load District",
		ShippingName:    "Saga Concurrency Tester",
		ShippingPhone:   "0987654321",
		ShippingFee:     30000.0,
		PaymentMethod:   1, // COD / standard
		Items: []CheckoutRequestItem{
			{
				ProductUUID: productUUID,
				Quantity:    1,
			},
		},
	}
	jsonPayload, _ := json.Marshal(reqBody)

	var successCount int64
	var outOfStockCount int64
	var otherErrorCount int64

	startSignal := make(chan struct{})
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 1; i <= concurrentUsers; i++ {
		wg.Add(1)
		idemKey := fmt.Sprintf("load-%s-%03d", testRunID, i)

		go func(key string) {
			defer wg.Done()

			// Block until firing gun
			<-startSignal

			req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/api/v1/orders", bytes.NewReader(jsonPayload))
			if reqErr != nil {
				atomic.AddInt64(&otherErrorCount, 1)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("x-Api-Key", os.Getenv("API_KEY"))
			req.Header.Set("X-Idempotency-Key", key)

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&otherErrorCount, 1)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			bodyBytes, _ := io.ReadAll(resp.Body)

			if resp.StatusCode == http.StatusCreated {
				atomic.AddInt64(&successCount, 1)
			} else {
				if bytes.Contains(bodyBytes, []byte("OUT_OF_STOCK")) ||
					bytes.Contains(bodyBytes, []byte("out of stock")) ||
					bytes.Contains(bodyBytes, []byte("insufficient stock")) ||
					bytes.Contains(bodyBytes, []byte("not enough stock")) ||
					resp.StatusCode == http.StatusConflict {
					atomic.AddInt64(&outOfStockCount, 1)
				} else {
					fmt.Printf("[UNEXPECTED ERROR %d]: %s\n", resp.StatusCode, string(bodyBytes))
					atomic.AddInt64(&otherErrorCount, 1)
				}
			}
		}(idemKey)
	}

	// Trigger all requests at once!
	close(startSignal)
	wg.Wait()
	duration := time.Since(startTime)

	// 6. Verification and Invariant Checks
	var finalStock int32
	_ = invPool.QueryRow(ctx, "SELECT quantity FROM inventory_stocks WHERE product_id = $1", productID).Scan(&finalStock)

	var confirmedSagas int64
	var failedSagas int64
	_ = orderPool.QueryRow(ctx, "SELECT count(*) FROM order_sagas WHERE idempotency_key LIKE $1 AND state = 'CONFIRMED'", "load-"+testRunID+"-%").Scan(&confirmedSagas)
	_ = orderPool.QueryRow(ctx, "SELECT count(*) FROM order_sagas WHERE idempotency_key LIKE $1 AND state = 'FAILED'", "load-"+testRunID+"-%").Scan(&failedSagas)

	fmt.Printf("\n====================================================================\n")
	fmt.Printf("📊 LOAD TEST EXECUTION RESULTS (Duration: %v)\n", duration)
	fmt.Printf("====================================================================\n")
	fmt.Printf("• Total Requests Sent     : %d\n", concurrentUsers)
	fmt.Printf("• Successful Checkouts    : %d  (Expected: %d)\n", successCount, testInitialStock)
	fmt.Printf("• Out Of Stock Rejections : %d  (Expected: %d)\n", outOfStockCount, concurrentUsers-testInitialStock)
	fmt.Printf("• Other Errors            : %d\n", otherErrorCount)
	fmt.Printf("--------------------------------------------------------------------\n")
	fmt.Printf("• Initial Stock           : %d\n", testInitialStock)
	fmt.Printf("• Final In-DB Stock       : %d  (Must be EXACTLY 0)\n", finalStock)
	fmt.Printf("• Confirmed Sagas in DB   : %d\n", confirmedSagas)
	fmt.Printf("• Failed Sagas in DB      : %d\n", failedSagas)
	fmt.Printf("====================================================================\n")

	// 7. Pass / Fail Evaluation
	passed := true
	if successCount != int64(testInitialStock) {
		fmt.Printf("❌ INVARIANT VIOLATION: Expected %d successes, got %d!\n", testInitialStock, successCount)
		passed = false
	}
	if finalStock != 0 {
		fmt.Printf("❌ OVERSELL DETECTED: Final stock is %d (must be 0)!\n", finalStock)
		passed = false
	}
	if confirmedSagas != int64(testInitialStock) {
		fmt.Printf("❌ INVARIANT VIOLATION: Expected %d confirmed sagas, got %d!\n", testInitialStock, confirmedSagas)
		passed = false
	}

	if passed {
		fmt.Printf("\n🎉 ALL INVARIANTS PASSED! ZERO OVERSELL PROVEN UNDER HIGH CONCURRENCY!\n\n")
	} else {
		fmt.Printf("\n💥 CONCURRENCY TEST FAILED!\n\n")
		os.Exit(1)
	}
}
