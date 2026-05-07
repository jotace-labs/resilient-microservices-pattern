package main

import (
	"context"
	"flag"
	"time"

	"github.com/joseCarlosAndrade/resilient-microservices-pattern/loadbalancer"
)

var (
	loader    = flag.Bool("loader", false, "run load test")
	targets   = flag.String("targets", "http://localhost:3001,http://localhost:4000,http://localhost:5000", "comma-separated list of target URLs")
	duration  = flag.Duration("duration", 60*time.Second, "how long to run the load test")
	workers   = flag.Int("workers", 10, "number of concurrent workers")
	rps       = flag.Int("rps", 100, "target requests per second (0 = unlimited)")
	endpoints = flag.String("endpoints", "/,/health", "comma-separated endpoints to hit")
)


func main() {

	flag.Parse()
	if *loader {
		targetList := parseCSV(*targets)
		endpointList := parseCSV(*endpoints)
		load(targetList, endpointList, *duration, *workers, *rps)
	} else {
		_Lb()
	}

	// _cb()

	
}

func _Lb() {
	go loadbalancer.StartServers()

	time.Sleep(1*time.Second)

	sp := loadbalancer.NewServerPool("http://localhost:3001", "http://localhost:4000", "http://localhost:5000")
	sp.StartServer(context.Background(), 8080)


}