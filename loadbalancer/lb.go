package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// todo
/*
https://kasvith.me/posts/lets-create-a-simple-lb-go/
*/

type Backend struct {
	id           int
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

// sets the backend state
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	defer b.mux.Unlock()

	b.Alive = alive
}

// returns the bakcend state
func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	defer b.mux.RUnlock()

	alive = b.Alive

	return
}

type ServerPool struct {
	backends []*Backend
	current  uint64
}

// attomically incrementing the counter and returning its value
func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

// GetNextPeer returns the next available peer to receive a connection
// it uses round robin
func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex()
	l := len(s.backends) + next // do a full cycle

	for i := next; i < l; i++ {
		idx := i % len(s.backends)

		if s.backends[idx].IsAlive() {
			if i != next { // just avoiding store the same id again in case its the next
				// if it goes through all the backends, go back to the same one
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

func (s *ServerPool) healthCheck(ctx context.Context) {
	// for each backend, check its readynez
	for i := 0; i < len(s.backends); i++ {

		host := "http://" + s.backends[i].URL.Host + "/readyz" // readyz responds if the application is ready to receive any traffic

		status, err := getHTTP(ctx, host)
		if err != nil {
			log.Printf("error trying to get readyz from: %s. err: %s", host, err.Error())
			s.backends[i].SetAlive(false)

		} else {
			if status != http.StatusOK {
				log.Printf("error getting readyz from %s: status %v", host, status)
				s.backends[i].SetAlive(false)

				continue
			}

			// health
			s.backends[i].SetAlive(true)
		}
	}
}

func (s *ServerPool) startHealthCheck(ctx context.Context) {
	t := time.NewTicker(time.Second * 8) // checks health each 15 seconds

	for {
		select {
		case <-t.C:
			log.Println("staring healthcheck")
			s.healthCheck(ctx)
			log.Println("finished healthcheck")
		case <-ctx.Done():
			log.Printf("closing healthcheck")
			return
		}
	}
}

// this would be enough
func (s *ServerPool) LbHandler() func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		peer := s.GetNextPeer()
		if peer != nil {
			log.Printf("forwaring to: %v: %s", peer.id, peer.URL.Host)
			peer.ReverseProxy.ServeHTTP(w, r)
			return
		}
		http.Error(w, "service not available", http.StatusServiceUnavailable)
	}
}

func (s *ServerPool) StartServer(ctx context.Context, port int) {
	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(s.LbHandler()),
	}

	go s.startHealthCheck(ctx)

	log.Printf("starting server on: localhost:%d", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

var serverPool *ServerPool

func NewServerPool(urls... string) (*ServerPool) {
	if len(urls) == 0 {
		log.Fatal("empty list of servers")
	}

	serverPool = &ServerPool{
		backends: make([]*Backend, 0),
		current: 0,
	}

	backends := make([]*Backend, 0)

	for id, urlStr  := range urls {
		serverUrl, err := url.Parse(urlStr)
		log.Printf("adding: urlStr: %s -> serverUrl: %s", urlStr, serverUrl.Host)
		if err != nil {
			log.Fatalf("could not add backend: %v", err.Error())
		}

		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
			log.Printf("error redirecting: [%s] %s\n", serverUrl.Host, e.Error())
			serverPool.backends[id].SetAlive(false)
			
		}

		b := &Backend{
			id: id,
			URL: serverUrl,
			ReverseProxy: proxy,
			Alive: true,
		}

		backends = append(backends, b)

	}

	serverPool.backends = backends

	return serverPool
}

