// Package main provides a unit test utility for rate limiting functionality.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"QuotaLane/internal/biz"
	"QuotaLane/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// Manual integration test for rate limiting functionality
// This tests the RateLimiterUseCase directly with a real Redis instance

func main() {
	// Create logger
	logger := log.NewStdLogger(os.Stdout)

	// Connect to Redis
	fmt.Println("==========================================")
	fmt.Println("QuotaLane Rate Limiting Integration Test")
	fmt.Println("==========================================")
	fmt.Println()

	fmt.Println("Step 1: Connect to Redis")
	fmt.Println("------------------------------------------")

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Printf("✗ Failed to connect to Redis: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Redis successfully")
	fmt.Println()

	// Create rate limit repo
	repo := data.NewRateLimitRepo(rdb, logger)
	rateLimiter := biz.NewRateLimiterUseCase(repo, logger)

	const accountID int64 = 99999 // Test account ID
	const rpmLimit int32 = 3
	const tpmLimit int32 = 100

	// Clean up test data
	defer func() {
		fmt.Println()
		fmt.Println("==========================================")
		fmt.Println("Cleanup")
		fmt.Println("==========================================")
		rdb.Del(ctx, fmt.Sprintf("rate:%d:rpm", accountID))
		rdb.Del(ctx, fmt.Sprintf("rate:%d:tpm", accountID))
		rdb.Del(ctx, fmt.Sprintf("concurrency:%d", accountID))
		fmt.Println("✓ Cleaned up test data")
	}()

	// Test RPM Rate Limiting
	fmt.Println("Step 2: Test RPM Rate Limiting")
	fmt.Println("------------------------------------------")
	fmt.Printf("RPM Limit: %d requests/minute\n", rpmLimit)
	fmt.Println()

	rpmPassed := 0
	for i := 1; i <= 5; i++ {
		err := rateLimiter.CheckRPM(ctx, accountID, rpmLimit)

		if i <= int(rpmLimit) {
			// Should pass
			if err == nil {
				fmt.Printf("  Request %d: ✓ PASS (expected)\n", i)
				rpmPassed++
			} else {
				fmt.Printf("  Request %d: ✗ FAIL - %v (expected PASS)\n", i, err)
			}
		} else {
			// Should fail
			if err != nil {
				fmt.Printf("  Request %d: ✓ BLOCKED - %v (expected)\n", i, err)
				rpmPassed++
			} else {
				fmt.Printf("  Request %d: ✗ FAIL - allowed (expected BLOCKED)\n", i)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	if rpmPassed == 5 {
		fmt.Println()
		fmt.Println("✓ RPM rate limiting works correctly!")
	} else {
		fmt.Println()
		fmt.Printf("✗ RPM test failed: %d/5 passed\n", rpmPassed)
	}
	fmt.Println()

	// Wait for window reset
	fmt.Println("Step 3: Wait for RPM window reset (60 seconds)...")
	fmt.Println("------------------------------------------")
	for i := 60; i > 0; i-- {
		fmt.Printf("\r  Waiting... %02d seconds remaining", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println()
	fmt.Println("✓ RPM window reset")
	fmt.Println()

	// Test TPM Rate Limiting
	fmt.Println("Step 4: Test TPM Rate Limiting")
	fmt.Println("------------------------------------------")
	fmt.Printf("TPM Limit: %d tokens/minute\n", tpmLimit)
	fmt.Println()

	tpmPassed := 0

	// First request: 40 tokens
	err := rateLimiter.CheckTPM(ctx, accountID, tpmLimit, 40)
	if err == nil {
		fmt.Println("  Request 1 (40 tokens): ✓ PASS (40/100 used)")
		tpmPassed++
	} else {
		fmt.Printf("  Request 1 (40 tokens): ✗ FAIL - %v\n", err)
	}

	// Second request: 40 tokens
	err = rateLimiter.CheckTPM(ctx, accountID, tpmLimit, 40)
	if err == nil {
		fmt.Println("  Request 2 (40 tokens): ✓ PASS (80/100 used)")
		tpmPassed++
	} else {
		fmt.Printf("  Request 2 (40 tokens): ✗ FAIL - %v\n", err)
	}

	// Third request: 30 tokens - should fail (would exceed 100)
	err = rateLimiter.CheckTPM(ctx, accountID, tpmLimit, 30)
	if err != nil {
		fmt.Printf("  Request 3 (30 tokens): ✓ BLOCKED - %v (expected)\n", err)
		tpmPassed++
	} else {
		fmt.Println("  Request 3 (30 tokens): ✗ FAIL - allowed (expected BLOCKED)")
	}

	// Fourth request: 10 tokens - should pass
	err = rateLimiter.CheckTPM(ctx, accountID, tpmLimit, 10)
	if err == nil {
		fmt.Println("  Request 4 (10 tokens): ✓ PASS (90/100 used)")
		tpmPassed++
	} else {
		fmt.Printf("  Request 4 (10 tokens): ✗ FAIL - %v\n", err)
	}

	if tpmPassed == 4 {
		fmt.Println()
		fmt.Println("✓ TPM rate limiting works correctly!")
	} else {
		fmt.Println()
		fmt.Printf("✗ TPM test failed: %d/4 passed\n", tpmPassed)
	}
	fmt.Println()

	// Test Concurrency Control
	fmt.Println("Step 5: Test Concurrency Control")
	fmt.Println("------------------------------------------")
	fmt.Println("Max concurrency: 10 requests")
	fmt.Println()

	concurrencyPassed := 0
	requestIDs := make([]string, 12)

	// Acquire 12 slots (should allow 10, block 2)
	for i := 0; i < 12; i++ {
		requestID := fmt.Sprintf("req-%d", i+1)
		requestIDs[i] = requestID

		err := rateLimiter.AcquireConcurrencySlot(ctx, accountID, requestID)

		if i < 10 {
			// Should succeed
			if err == nil {
				fmt.Printf("  Request %2d: ✓ ACQUIRED slot\n", i+1)
				concurrencyPassed++
			} else {
				fmt.Printf("  Request %2d: ✗ FAIL - %v (expected ACQUIRED)\n", i+1, err)
			}
		} else {
			// Should fail
			if err != nil {
				fmt.Printf("  Request %2d: ✓ BLOCKED - %v (expected)\n", i+1, err)
				concurrencyPassed++
			} else {
				fmt.Printf("  Request %2d: ✗ FAIL - allowed (expected BLOCKED)\n", i+1)
			}
		}
	}

	fmt.Println()

	// Release first 5 slots
	fmt.Println("Releasing 5 concurrent slots...")
	for i := 0; i < 5; i++ {
		_ = rateLimiter.ReleaseConcurrencySlot(ctx, accountID, requestIDs[i])
		fmt.Printf("  Released slot for request %d\n", i+1)
	}
	fmt.Println()

	// Try to acquire again (should succeed now)
	err = rateLimiter.AcquireConcurrencySlot(ctx, accountID, "req-13")
	if err == nil {
		fmt.Println("  Request 13: ✓ ACQUIRED slot (after release)")
		concurrencyPassed++
	} else {
		fmt.Printf("  Request 13: ✗ FAIL - %v (expected ACQUIRED)\n", err)
	}

	// Release remaining slots
	for i := 5; i < 10; i++ {
		_ = rateLimiter.ReleaseConcurrencySlot(ctx, accountID, requestIDs[i])
	}
	_ = rateLimiter.ReleaseConcurrencySlot(ctx, accountID, "req-13")

	if concurrencyPassed == 13 {
		fmt.Println()
		fmt.Println("✓ Concurrency control works correctly!")
	} else {
		fmt.Println()
		fmt.Printf("✗ Concurrency test failed: %d/13 passed\n", concurrencyPassed)
	}
	fmt.Println()

	// Test Token Estimation
	fmt.Println("Step 6: Test Token Estimation")
	fmt.Println("------------------------------------------")

	testCases := []struct {
		prompt      string
		maxOutput   int32
		expectedMin int32
		expectedMax int32
	}{
		{"Hello, world!", 100, 100, 105},
		{"This is a test prompt with about 40 characters.", 200, 205, 215},
		{"", 50, 50, 50},
	}

	estimationPassed := 0
	for i, tc := range testCases {
		estimated := rateLimiter.EstimateTokens(tc.prompt, tc.maxOutput)
		if estimated >= tc.expectedMin && estimated <= tc.expectedMax {
			fmt.Printf("  Test %d: ✓ PASS - estimated %d tokens (expected %d-%d)\n",
				i+1, estimated, tc.expectedMin, tc.expectedMax)
			estimationPassed++
		} else {
			fmt.Printf("  Test %d: ✗ FAIL - estimated %d tokens (expected %d-%d)\n",
				i+1, estimated, tc.expectedMin, tc.expectedMax)
		}
	}

	if estimationPassed == len(testCases) {
		fmt.Println()
		fmt.Println("✓ Token estimation works correctly!")
	} else {
		fmt.Println()
		fmt.Printf("✗ Token estimation test failed: %d/%d passed\n", estimationPassed, len(testCases))
	}
	fmt.Println()

	// Summary
	fmt.Println("==========================================")
	fmt.Println("Test Summary")
	fmt.Println("==========================================")

	totalTests := 5 + 4 + 13 + len(testCases)
	totalPassed := rpmPassed + tpmPassed + concurrencyPassed + estimationPassed

	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Tests Passed: %d\n", totalPassed)
	fmt.Printf("Tests Failed: %d\n", totalTests-totalPassed)
	fmt.Println()

	if totalPassed == totalTests {
		fmt.Println("✓ All rate limiting tests completed successfully!")
		os.Exit(0)
	} else {
		fmt.Println("✗ Some tests failed. Please review the output above.")
		os.Exit(1)
	}
}
