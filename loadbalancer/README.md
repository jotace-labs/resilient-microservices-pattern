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
  