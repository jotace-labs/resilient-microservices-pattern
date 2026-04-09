# Load Balancer

Using simple round-robin algorithm to balance load across pre-defined static backends. Test it with:

```bash
curl localhost:8000
```

The servers have a 40% chance of replying with Not Ok for the `/readyz` endpoint, so we can test which server keeps responding.
