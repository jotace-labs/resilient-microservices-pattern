package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)



func load(targetList []string, endpointList []string, duration time.Duration, workers int, rps int) {
	

	if len(targetList) == 0 || len(endpointList) == 0 {
		log.Fatal("targets and endpoints cannot be empty")
	}

	fmt.Printf("Load test config:\n")
	fmt.Printf("  Targets: %v\n", targetList)
	fmt.Printf("  Duration: %s\n", duration)
	fmt.Printf("  Workers: %d\n", workers)
	fmt.Printf("  Target RPS: %d\n", rps)
	fmt.Printf("  Endpoints: %v\n", endpointList)
	fmt.Printf("\n")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var (
		totalRequests int64
		totalErrors   int64
	)

	ctx := time.After(duration)
	var wg sync.WaitGroup

	// rate limiter channel if rps > 0
	var limiter <-chan time.Time
	if rps > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(rps))
		limiter = ticker.C
		defer ticker.Stop()
	}

	// start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx:
					return
				default:
					if 	rps > 0 {
						<-limiter
					}

					target := targetList[rand.Intn(len(targetList))]
					endpoint := endpointList[rand.Intn(len(endpointList))]
					url := target + endpoint

					resp, err := client.Get(url)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						continue
					}

					// consume response body
					io.ReadAll(resp.Body)
					resp.Body.Close()

					atomic.AddInt64(&totalRequests, 1)
					if resp.StatusCode != http.StatusOK {
						atomic.AddInt64(&totalErrors, 1)
					}
				}
			}
		}(i)
	}

	// stats ticker
	statsTicker := time.NewTicker(5 * time.Second)
	defer statsTicker.Stop()

	go func() {
		for range statsTicker.C {
			reqs := atomic.LoadInt64(&totalRequests)
			errs := atomic.LoadInt64(&totalErrors)
			successRate := float64(reqs-errs) / float64(reqs) * 100
			if reqs == 0 {
				successRate = 0
			}
			fmt.Printf("[STATS] Requests: %d, Errors: %d, Success Rate: %.1f%%\n", reqs, errs, successRate)
		}
	}()

	wg.Wait()
	fmt.Printf("\n=== Final Results ===\n")
	totalReqs := atomic.LoadInt64(&totalRequests)
	totalErrs := atomic.LoadInt64(&totalErrors)
	fmt.Printf("Total Requests: %d\n", totalReqs)
	fmt.Printf("Total Errors: %d\n", totalErrs)
	if totalReqs > 0 {
		fmt.Printf("Success Rate: %.2f%%\n", float64(totalReqs-totalErrs)/float64(totalReqs)*100)
	}
}

func parseCSV(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
