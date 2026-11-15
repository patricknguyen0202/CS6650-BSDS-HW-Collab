"""Read-after-write and concurrent-update tests against the cart API.
Usage: python3 consistency_test.py --host http://<ALB> --label eventual --out ../results/consistency_eventual.json"""
import argparse, json, threading, time, urllib.error, urllib.request

p = argparse.ArgumentParser()
p.add_argument("--host", required=True)
p.add_argument("--label", required=True)
p.add_argument("--out", required=True)
p.add_argument("--n", type=int, default=50)
a = p.parse_args()
base = a.host.rstrip("/")


def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return r.status, json.loads(r.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, None


def read_until(path, check, timeout=5.0):
    """Immediate read, then retry every 10 ms. Returns (consistent_on_first_read, ms_until_consistent)."""
    start, first = time.perf_counter(), True
    while time.perf_counter() - start < timeout:
        s, body = call("GET", path)
        if check(s, body):
            return first, (time.perf_counter() - start) * 1000
        first = False
        time.sleep(0.01)
    return False, None


results = {"label": a.label}
# 1) create -> immediately get
stale, waits = 0, []
for _ in range(a.n):
    s, body = call("POST", "/shopping-carts", {"customer_id": 1})
    cid = body["shopping_cart_id"]
    first, ms = read_until(f"/shopping-carts/{cid}", lambda s, b: s == 200)
    stale += 0 if first else 1
    waits.append(ms)
ok_waits = [w for w in waits if w is not None]
results["create_then_get"] = {"runs": a.n, "stale_first_reads": stale, "never_consistent": a.n - len(ok_waits),
                              "max_ms_to_consistent": round(max(ok_waits), 1) if ok_waits else None}

# 2) add item -> immediately get
s, body = call("POST", "/shopping-carts", {"customer_id": 2})
cid = body["shopping_cart_id"]
stale = 0
for i in range(1, a.n + 1):
    call("POST", f"/shopping-carts/{cid}/items", {"product_id": i, "quantity": 1})
    first, _ = read_until(f"/shopping-carts/{cid}", lambda s, b, i=i: s == 200 and any(x["product_id"] == i for x in b["items"]))
    stale += 0 if first else 1
results["add_then_get"] = {"runs": a.n, "stale_first_reads": stale}

# 3) rapid concurrent updates to the same cart: 10 clients x 20 adds of product 1
s, body = call("POST", "/shopping-carts", {"customer_id": 3})
cid = body["shopping_cart_id"]
def worker():
    for _ in range(20):
        call("POST", f"/shopping-carts/{cid}/items", {"product_id": 1, "quantity": 1})
threads = [threading.Thread(target=worker) for _ in range(10)]
[t.start() for t in threads]
[t.join() for t in threads]
time.sleep(1)
s, body = call("GET", f"/shopping-carts/{cid}")
final = next((x["quantity"] for x in body["items"] if x["product_id"] == 1), 0)
results["concurrent_updates"] = {"expected_quantity": 200, "final_quantity": final, "lost_updates": 200 - final}

json.dump(results, open(a.out, "w"), indent=2)
print(json.dumps(results, indent=2))
