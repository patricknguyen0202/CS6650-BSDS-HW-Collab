import sys
import time
import json
import statistics
import requests
from datetime import datetime, timezone

BASE_URL = "http://hw8-shopping-cart-alb-688672353.us-east-1.elb.amazonaws.com"
ITERATIONS = 50
OUTPUT_FILE = "mysql_test_results.json"

results = []
cart_ids = []
session = requests.Session()

def record_op(operation_name, start_time, response, expected_status):
    elapsed_ms = round((time.perf_counter() - start_time) * 1000, 2)
    timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    success = (response.status_code == expected_status)
    
    results.append({
        "operation": operation_name,
        "response_time": elapsed_ms,
        "success": success,
        "status_code": response.status_code,
        "timestamp": timestamp
    })
    return success, elapsed_ms

print(f"Starting 150-operation MySQL test against: {BASE_URL}")
test_start = time.perf_counter()

# 1. 50 CREATE_CART Operations (POST /shopping-carts)
print("\n[1/3] Running 50 create_cart operations...")
for i in range(1, ITERATIONS + 1):
    payload = {"customer_id": 1000 + i}
    start = time.perf_counter()
    resp = session.post(f"{BASE_URL}/shopping-carts", json=payload, timeout=10)
    ok, _ = record_op("create_cart", start, resp, 201)
    if ok:
        cart_ids.append(resp.json()["shopping_cart_id"])
    else:
        print(f"  [!] create_cart #{i} failed: {resp.status_code} {resp.text}")
        cart_ids.append(i)

# 2. 50 ADD_ITEMS Operations (POST /shopping-carts/{id}/items)
print("[2/3] Running 50 add_items operations...")
for i, cid in enumerate(cart_ids, start=1):
    payload = {"product_id": 200 + (i % 25), "quantity": (i % 5) + 1}
    start = time.perf_counter()
    resp = session.post(f"{BASE_URL}/shopping-carts/{cid}/items", json=payload, timeout=10)
    ok, _ = record_op("add_items", start, resp, 204)
    if not ok:
        print(f"  [!] add_items #{i} (cart {cid}) failed: {resp.status_code} {resp.text}")

# 3. 50 GET_CART Operations (GET /shopping-carts/{id})
print("[3/3] Running 50 get_cart operations...")
for i, cid in enumerate(cart_ids, start=1):
    start = time.perf_counter()
    resp = session.get(f"{BASE_URL}/shopping-carts/{cid}", timeout=10)
    ok, _ = record_op("get_cart", start, resp, 200)
    if not ok:
        print(f"  [!] get_cart #{i} (cart {cid}) failed: {resp.status_code} {resp.text}")

total_duration = round(time.perf_counter() - test_start, 2)

with open(OUTPUT_FILE, "w") as f:
    json.dump(results, f, indent=2)

def percentile(sorted_list, pct):
    k = (len(sorted_list) - 1) * (pct / 100.0)
    f_idx = int(k)
    c_idx = min(f_idx + 1, len(sorted_list) - 1)
    if f_idx == c_idx:
        return round(sorted_list[f_idx], 2)
    return round(sorted_list[f_idx] * (c_idx - k) + sorted_list[c_idx] * (k - f_idx), 2)

all_times = sorted(r["response_time"] for r in results)
success_count = sum(1 for r in results if r["success"])
success_rate = round((success_count / len(results)) * 100, 2)

print("\n" + "="*55)
print(f"MYSQL TEST RESULTS SUMMARY (Completed in {total_duration}s)")
print("="*55)
print(f"Total Operations : {len(results)} (Required: 150)")
print(f"Success Rate     : {success_rate}% ({success_count}/{len(results)})")
print(f"Avg Response Time: {round(statistics.mean(all_times), 2)} ms")
print(f"P50 Response Time: {percentile(all_times, 50)} ms")
print(f"P95 Response Time: {percentile(all_times, 95)} ms")
print(f"P99 Response Time: {percentile(all_times, 99)} ms")
print(f"Min / Max Time   : {min(all_times)} ms / {max(all_times)} ms")
print("-" * 55)
print("OPERATION-SPECIFIC BREAKDOWN:")
for op in ["create_cart", "add_items", "get_cart"]:
    op_times = sorted(r["response_time"] for r in results if r["operation"] == op)
    print(f"  {op.upper():<12} -> Avg: {round(statistics.mean(op_times), 2):>6} ms | "
          f"P50: {percentile(op_times, 50):>6} ms | P95: {percentile(op_times, 95):>6} ms")
print("="*55)
print(f"Saved 150 records to: {OUTPUT_FILE}")
