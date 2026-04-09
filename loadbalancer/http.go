package loadbalancer

import (
	"context"
	"fmt"
	"net/http"
	"time"
)


func getHTTP(ctx context.Context, url string) (int, error) {
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return 0, fmt.Errorf("failed to create request: %w", err)
    }

	c := http.Client{
		Timeout: time.Second*5,
	}
	
    resp, err := c.Do(req)
    if err != nil {
        return 0, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
 
    return resp.StatusCode, nil
}