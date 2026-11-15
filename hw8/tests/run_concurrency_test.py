import time
import statistics
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

BASE_URL = "http://hw8-shopping-cart-alb-688672353.us-east-1.elb.amazonaws.com"

# --- Test A: 50-Item Cart Retrieval Latency ---
print("\n[1/2] Testing 50-Item Cart Retrieval...")
s = requests.Session()
r = s.post(f"{BASE_URL}/shopping-carts", json={"customer_id": 9999}, timeout=10)
big_cart_id = r.json()["shopping_cart_id"]

for prod_id in range(1, 51):
    s.post(f"{BASE_URL}/shopping-carts/{big_cart_id}/items", json={"product_id": prod_id, "quantity": 1}, timeout=10)

get_50_times = []
for _ in range(20):
    t0 = time.perf_counter()
    resp = s.get(f"{BASE_URL}/shopping-carts/{big_cart_id}", timeout=10)
    get_50_times.append((time.perf_counter() - t0) * 1000)

items_count = len(resp.json().get("items", []))
get_50_times.sort()
print(f"  -> Cart #{big_cart_id} populated with {items_count} distinct items.")
print(f"  -> 50-Item GET Avg: {round(statistics.mean(get_50_times), 2)} ms | "
      f"P50: {round(statistics.median(get_50_times), 2)} ms | "
      f"P95: {round(get_50_times[18], 2)} ms")

# --- Test B: 100 Concurrent Shopping Sessions ---
print("\n[2/2] Testing 100 Concurrent Shopping Sessions (300 total requests)...")
def simulate_session(user_idx):
    sess = requests.Session()
    t0 = time.perf_counter()
    c_resp = sess.post(f"{BASE_URL}/shopping-carts", json={"customer_id": 5000 + user_idx}, timeout=15)
    if c_resp.status_code != 201:
        return False, 0
    cid = c_resp.json()["shopping_cart_id"]
    a_resp = sess.post(f"{BASE_URL}/shopping-carts/{cid}/items", json={"product_id": 10, "quantity": 2}, timeout=15)
    g_resp = sess.get(f"{BASE_URL}/shopping-carts/{cid}", timeout=15)
    total_ms = (time.perf_counter() - t0) * 1000
    return (a_resp.status_code == 204 and g_resp.status_code == 200), total_ms

start_wall = time.perf_counter()
with ThreadPoolExecutor(max_workers=100) as pool:
    futures = [pool.submit(simulate_session, i) for i in range(1, 101)]
    results = [f.result() for f in as_completed(futures)]

wall_time = round(time.perf_counter() - start_wall, 2)
successes = sum(1 for ok, _ in results if ok)
session_times = sorted(t for ok, t in results if ok)

print(f"  -> Completed 100 concurrent sessions in {wall_time}s")
print(f"  -> Success Rate: {successes}/100 ({successes}%)")
print(f"  -> Avg Session Duration (3 ops): {round(statistics.mean(session_times), 2)} ms | "
      f"P95: {round(session_times[int(len(session_times)*0.95)], 2)} ms\n")
