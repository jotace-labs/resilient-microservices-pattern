package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cb "github.com/joseCarlosAndrade/resilient-microservices-pattern/circuitbreaker"
)

type CustomService struct {
	httpClient *http.Client
	circuitBreaker *cb.CircuitBreaker

	baseURL string
	// todo: add metrics
}

func NewCustomService(baseURL string) *CustomService {
	cfg := cb.Config{
		HalfOpenMaxRequests: 5,
		Interval: 10 * time.Second,
		OpenStateTimeout: 30 * time.Second,
		ReadyToTrip: func(counts cb.Counts) bool { // custom function to determine if the circuit should open
			failureRation := float64(counts.TotalFailures)/float64(counts.Requests)
			
			return failureRation >= 0.5 && counts.Requests >= 5
		
		},
		OnStateChange: func(name string, from cb.State, to cb.State) {
			fmt.Printf("cb: %s transitioning from %s to %s\n", name, from.String(), to.String())
		},
	}

	cb := cb.NewCircuitBreaker("custom-service", cfg)
	return &CustomService{
		httpClient: &http.Client{
			Timeout: 5*time.Second,
		},
		circuitBreaker: cb,
		baseURL: baseURL,
	}
}

func (s *CustomService) fetchContent(ctx context.Context, contentID string) (interface{}, error) {
    url := fmt.Sprintf("%s/content/%s", s.baseURL, contentID)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
 
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
 
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }
 
    var content interface{}
    if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }
 
    return &content, nil
}

func (s *CustomService) GetContent(ctx context.Context, contentID string) (interface{}, error) {
	result, err := s.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		return s.fetchContent(ctx, contentID)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	return result, err // should do casting here
}


func _cb() {
	service := NewCustomService("http://localhost:8080")

	for i := 0; i < 20; i++ {
		content, err := service.GetContent(context.Background(), fmt.Sprintf("content-%d", i))
		if err != nil {
			fmt.Printf("failed to get content: %v\n", err)
		} else {
			fmt.Printf("got content: %v\n", content)
		}

		time.Sleep(1 * time.Second)
	}
}