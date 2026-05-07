# Load Balancer

Using simple round-robin algorithm to balance load across pre-defined static backends. Test it with:

```bash
curl localhost:8000
```

The servers have a 40% chance of replying with Not Ok for the `/readyz` endpoint, so we can test which server keeps responding.

## todo

- insert metrics to measure: average latency, how many requests were processed, how many times each server served the request, failure rate, etc
  - prometheus
  - grafana

- test scripts
  - request generator with heterogeneous payload
  - make servers actually process them
  - also instrument them
  

## Instrumentating

### Prometheus

`/metrics` gets us something like:

```text
# HELP api_http_request_duration_seconds Duration of HTTP requests in seconds
# TYPE api_http_request_duration_seconds histogram
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.005"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.01"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.025"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.05"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.1"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.25"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="0.5"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="1"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="2.5"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="5"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="10"} 17
api_http_request_duration_seconds_bucket{method="GET",path="/user",status_code="200",le="+Inf"} 17
api_http_request_duration_seconds_sum{method="GET",path="/user",status_code="200"} 0.019494052999999994
api_http_request_duration_seconds_count{method="GET",path="/user",status_code="200"} 17
# HELP api_http_request_total Total number of HTTP requests
# TYPE api_http_request_total counter
api_http_request_total{method="GET",path="/user",status_code="200"} 17
```

prometheus create buckets for each combination of labels: path, method and status_code, so we can analyse each metrics individually.

le= less than or equal to, which is the boundary of the histogram

### api_http_request_duration_seconds

bucket histogram that tracks how long each request takes broken down by path, method and status code

Get total request count per endpoint

```text
sum by(path, method) (increase(api_http_request_duration_seconds_count[5m]))
```

ex: total request count for `GET /users` over the last 5 minutes.

---

Get average request latency foer each method;path

```text
sum by(path, method) (rate(api_http_request_duration_seconds_sum[5m])) 
/ 
sum by(path, method) (rate(api_http_request_duration_seconds_count[5m]))
```

sums the duration over the last 5 minutes and divide by count of request over the last 5 minutes

---

95 percentile latency (p95)

```text
histogram_quantile(0.95, sum by(le, path, method) (rate(api_http_request_duration_seconds_bucket[5m])))
```

95% of the request to this method;path are faster than this result

---

latency distribution for each le

```text
sum by(le) (rate(api_http_request_duration_seconds_bucket[1m]))
```

---

top 5 slowest endpoints by p95 latency

```text
topk(5, histogram_quantile(0.95, sum by(le, path) (rate(api_http_request_duration_seconds_bucket[5m]))))
```
