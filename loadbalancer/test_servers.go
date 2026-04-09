package loadbalancer

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func startTestServer(port int, name string) {
	mux := http.NewServeMux()

	// /user endpoint - returns 200 OK with a server name
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hello from %s on port %d\n", name, port)
	})

	// /readyz endpoint - returns 200 OK 60% of the time
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if rand.Float64() < 0.6 {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Ready\n")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Not ready\n")
		}
	})

	addr := fmt.Sprintf("localhost:%d", port)
	log.Printf("Starting %s on %s", name, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Failed to start server on %s: %v", addr, err)
	}
}

func StartServers() {
	rand.Seed(time.Now().UnixNano())

	// Start three servers in goroutines
	go startTestServer(3000, "Server-A")
	go startTestServer(4000, "Server-B")
	go startTestServer(5000, "Server-C")

	// Keep the program running
	select {}
}
