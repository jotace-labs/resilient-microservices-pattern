package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

/*
study from: https://tskulbru.dev/posts/building-resilient-microservices-k8s-circuit-breakers-retries-chaos-engineering/

closed: the circuit breaker is working fine and allowing requests to pass through. If the error rate exceeds a certain threshold, the circuit breaker will transition to open state

open: the circuit breaker is not working and all calls fail immediately. The circuit breaker will transition to half-open state after a certain timeout

half-open: the circuit breaker is testing if the service is back to normal. It allows a limited number of requests to pass through and if they succeed, it transitions back to closed state. If they fail, it transitions back to open state
*/

type State int

const (
	StateClosed State = iota // working fine
	StateOpen // not working, all calls fail immediately
	StateHalfOpen // testing if the service is back to normal
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type Config struct {
	HalfOpenMaxRequests uint32
	Interval time.Duration
	OpenStateTimeout time.Duration	
	ReadyToTrip func(counts Counts) bool // optional function to determine if the circuit should trip. If not provided, the default is to trip when the error rate is 50% or more
	OnStateChange func(name string, from State, to State) // optional function to be called when the state changes
}

type Counts struct {
	Requests uint32
	TotalSuccesses uint32
	TotalFailures uint32
	ConsecutiveSuccesses uint32 // Consecutive successes and failures are used to determine when to transition from half-open to closed or open states, respectively
	ConsecutiveFailures uint32
}

type CircuitBreaker struct {
	name string
	halfOpenMaxRequests uint32 // halfOpenMaxRequests is the maximum number of requests allowed in half-open state before transitioning to open state. This is used to prevent a thundering herd of requests from overwhelming the service when it transitions to half-open state
	interval time.Duration // interval is the time window for the counts of requests, successes, and failures. This is used to determine when to transition from closed to open state based on the error rate
	openStateTimeout time.Duration 
	readyToTrip func(counts Counts) bool // readyToTrip is a function that determines when to trip the circuit breaker from closed to open state based on the counts of requests, successes, and failures. If not provided, the default is to trip when the error rate is 50% or more
	onStateChange func(name string, from State, to State) // optional function to be called when the state changes

	state State // current state of the circuit breaker
	counts Counts // counts of requests, successes, and failures
	generation uint64 // generation is used to track the number of times the circuit breaker has transitioned to open state. This is used to prevent old requests from affecting the state of the circuit breaker after it has transitioned to open state
	expiry time.Time // expiry is the time when the circuit breaker will transition from open to half-open state
	mutex sync.Mutex // mutex is used to protect the state and counts of the circuit breaker from concurrent access
}

var (
	ErrOpenState = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

func NewCircuitBreaker(name string, config Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		name: name,
		halfOpenMaxRequests: config.HalfOpenMaxRequests,
		interval: config.Interval,
		openStateTimeout: config.OpenStateTimeout,
		readyToTrip: config.ReadyToTrip,
		onStateChange: config.OnStateChange,
		state: StateClosed,
	}

	if cb.halfOpenMaxRequests == 0 {
		cb.halfOpenMaxRequests = 5 // default max requests in half-open state
	}

	if cb.interval <= 0 {
		cb.interval = time.Duration(0) // default interval for counts
	}

	if cb.openStateTimeout <= 0 {
		cb.openStateTimeout = time.Duration(60) * time.Second // default timeout for open state
	}

	if cb.readyToTrip == nil {
		cb.readyToTrip = func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 5 // default readyToTrip function that trips the circuit breaker (opens it) when there are 5 or more consecutive failures
		}
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// toNewGeneration transitions the circuit breaker to a new generation and resets the counts. It also sets the expiry time based on the current state of the circuit breaker. This function is called when the circuit breaker transitions to a new state or when the interval for counts has passed in closed state
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time

	// set the expiry time based on the current state of the circuit breaker
	switch cb.state {
	case StateClosed: // in closed state, the expiry time is set to the current time plus the interval for counts. This means that the counts will be reset after the interval has passed, allowing the circuit breaker to transition to open state if there are too many failures within that interval
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen: // in open state, the expiry time is set to the current time plus the timeout for open state. This means that the circuit breaker will transition to half-open state after the timeout has passed, allowing it to test if the service is back to normal
		cb.expiry = now.Add(cb.openStateTimeout)
	case StateHalfOpen: // in half-open state, the expiry time is set to the zero value of time.Time, which means that it will not automatically transition to open state based on the interval for counts. Instead, it will transition to open state if there are too many failures within the max requests allowed in half-open state (halfOpenMaxRequests)
		cb.expiry = zero
	}
}

// setState sets the state of the circuit breaker and updates the generation and expiry time accordingly. It also calls the onStateChange function if it is provided
func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prevState := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prevState, state)
	}
}

// currentState returns the current state of the circuit breaker and the current generation. It also checks if the circuit breaker should transition to a new generation based on the current time and the expiry time. This function is called before each request to determine if the request should be allowed or rejected based on the current state of the circuit breaker
func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
		case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}

	return cb.state, cb.generation
}

// beforeRequest is called before each request to check if the request should be allowed or rejected based on the current state of the circuit breaker. It also increments the count of requests and returns the current generation. If the circuit breaker is in open state, it returns an error indicating that the circuit breaker is open. If the circuit breaker is in half-open state and the number of requests has reached the maximum allowed in half-open state, it returns an error indicating that there are too many requests in half-open state
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	
	// check if the circuit breaker should transition to a new generation based on the current time and the expiry time
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrOpenState
	
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.halfOpenMaxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++

	return generation, nil
}

// afterRequest is called after each request to update the counts of successes and failures based on the result of the request. It also checks if the circuit breaker should transition to a new state based on the updated counts. This function is called with the generation returned by beforeRequest to ensure that the counts are only updated for the current generation of the circuit breaker
func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before { // if the generation has changed since beforeRequest was called, it means that the circuit breaker has transitioned to a new state and the counts have been reset. In this case, we should not update the counts based on the result of the request, as it may not be relevant to the current state of the circuit breaker
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess updates the counts of successes and failures based on the result of a successful request. It also checks if the circuit breaker should transition from half-open to closed state based on the updated counts. This function is called by afterRequest when a request is successful
func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveSuccesses++
	cb.counts.ConsecutiveFailures = 0

	if state == StateHalfOpen {
		cb.setState(StateClosed, now)
	}
}

// onFailure updates the counts of successes and failures based on the result of a failed request. It also checks if the circuit breaker should transition from half-open to open state based on the updated counts. This function is called by afterRequest when a request fails
func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveFailures++
	cb.counts.ConsecutiveSuccesses = 0

	if cb.readyToTrip(cb.counts) {
		cb.setState(StateOpen, now)
	}
}

// Execute executes a request through the circuit breaker. It first calls beforeRequest to check if the request should be allowed or rejected based on the current state of the circuit breaker. If the request is allowed, it executes the request and then calls afterRequest to update the counts of successes and failures based on the result of the request. If the request is rejected, it returns an error indicating that the circuit breaker is open or that there are too many requests in half-open state
func (cb *CircuitBreaker) Execute(ctx context.Context, request func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err // if the circuit breaker is open or if there are too many requests in half-open state, we should return an error indicating that the request is rejected
	}

	defer func() {
		if r := recover(); r != nil { // if the request panics, we should treat it as a failure and update the counts accordingly. We also need to re-panic to propagate the panic to the caller
			cb.afterRequest(generation, false)
			panic(r)
		}
	} ()

	result, err := request()
	cb.afterRequest(generation, err == nil)

	return result, err
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
    cb.mutex.Lock()
    defer cb.mutex.Unlock()
    
    now := time.Now()
    state, _ := cb.currentState(now)
    return state
}
 
// GetCounts returns the current counts
func (cb *CircuitBreaker) GetCounts() Counts {
    cb.mutex.Lock()
    defer cb.mutex.Unlock()
    
    return cb.counts
}