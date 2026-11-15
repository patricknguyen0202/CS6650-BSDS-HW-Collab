"""HW8 required test: exactly 150 operations (50 create, 50 add items, 50 get), sequential, one client.
Usage: python3 perf_test.py --host http://<ALB> --out ../results/dynamodb_test_results.json"""
import argparse, json, random, statistics, time, urllib.error, urllib.request
from datetime import datetime, timezone

p = argparse.ArgumentParser()
p.add_argument("--host", required=True)
p.add_argument("--out", required=True)
a = p.parse_args()
base = a.host.rstrip("/")
records, cart_ids = [], []


def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method, headers={"Content-Type": "application/json"})
    start = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            status, payload = r.status, r.read()
    except urllib.error.HTTPError as e:
        status, payload = e.code, e.read()
    except Exception:
        status, payload = 0, b""
    return status, payload, (time.perf_counter() - start) * 1000


def record(op, ms, ok, status):
    records.append({"operation": op, "response_time": round(ms, 2), "success": ok, "status_code": status,
                    "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")})


t0 = time.time()
for _ in range(50):
    s, body, ms = call("POST", "/shopping-carts", {"customer_id": random.randint(1, 1000)})
    record("create_cart", ms, s == 201, s)
    if s == 201:
        cart_ids.append(json.loads(body)["shopping_cart_id"])
for i in range(50):
    cid = cart_ids[i % len(cart_ids)] if cart_ids else 1
    s, _, ms = call("POST", f"/shopping-carts/{cid}/items", {"product_id": random.randint(1, 1000), "quantity": random.randint(1, 5)})
    record("add_items", ms, s == 204, s)
for i in range(50):
    cid = cart_ids[i % len(cart_ids)] if cart_ids else 1
    s, _, ms = call("GET", f"/shopping-carts/{cid}")
    record("get_cart", ms, s == 200, s)

assert len(records) == 150
with open(a.out, "w") as f:
    json.dump(records, f, indent=2)
print(f"150 operations in {time.time() - t0:.1f} s -> {a.out}")
for op in ("create_cart", "add_items", "get_cart"):
    t = sorted(r["response_time"] for r in records if r["operation"] == op)
    ok = sum(r["success"] for r in records if r["operation"] == op)
    print(f"{op:12s} success {ok}/50  avg {statistics.mean(t):7.1f} ms  p50 {t[24]:7.1f}  p95 {t[47]:7.1f}  max {t[-1]:7.1f}")
