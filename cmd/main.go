package main

import (
	"context"
	"time"

	"github.com/joseCarlosAndrade/resilient-microservices-pattern/loadbalancer"
)


func main() {
	// _cb()
	_Lb()
}

func _Lb() {
	go loadbalancer.StartServers()

	time.Sleep(1*time.Second)

	sp := loadbalancer.NewServerPool("http://localhost:3000", "http://localhost:4000", "http://localhost:5000")
	sp.StartServer(context.Background(), 8080)
}