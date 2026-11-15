# STEP II Implementation Notes — DynamoDB (Part 1: Sushant Mudalgi · Parts 2–6: Anuj Chobe)

## Partition key strategy (Part 1, Sushant)
One table `hw8-shopping-cart-carts`, partition key `cart_id` (random positive int32, hashed by DynamoDB → even spread), no sort key,
items embedded as a map `{product_id: quantity}`, GSI `customer_id-index` for customer history, TTL `expires_at` (7 days), on-demand
billing, gateway VPC endpoint. Details: `DYNAMODB_DESIGN.md`.
**Under load (Part 6):** Contributor Insights showed ordinary carts at ~4 consumed units each; the only two hot keys (~600 and ~200
units) were the carts the consistency test hammers on purpose (200 concurrent adds; 50 adds with repeated read-backs). Peak usage stayed
under 2 units/s (~0% of max throughput), with no throttling recorded. Error metrics: 0 system errors (read and write), 0 transaction conflicts, and exactly 1 conditional-check failure: the deliberate smoke test "add item to a missing cart" (404 via `attribute_exists(cart_id)`). That count also shows no random `cart_id` collision occurred on create.

## Implementation (Part 2)
`src/dynamo.go` implements the three endpoints with one DynamoDB call each (PutItem / GetItem / conditional UpdateItem).
`CART_STORE=dynamodb` selects it in `main()`; any other value keeps the MySQL path unchanged (`cart_store` defaults to `mysql`).
Terraform passes `CART_STORE`, `DYNAMODB_TABLE`, `DYNAMODB_CONSISTENT_READ`, `AWS_REGION` to the task. Smoke tests return the same
codes as MySQL: 201 / 204 / 200 / 404 / 400; two adds of product 7 (2 + 3) read back as 5.

## Key differences from the MySQL implementation
| | MySQL (STEP I) | DynamoDB (STEP II) |
|---|---|---|
| Get cart | LEFT JOIN across 2 tables | 1 GetItem (items embedded) |
| Add item | transaction: check cart + upsert | 1 conditional UpdateItem, server-side increment |
| IDs | AUTO_INCREMENT | random int32 + `attribute_not_exists`, retry on collision |
| Connections | pool of 10 | stateless HTTPS calls, no pool |
| Consistency | strong | eventual by default; `ConsistentRead` optional (2× read cost) |
| 150-op avg latency (laptop → ALB) | see `mysql_test_results.json` (STEP III) | create 77.5 ms · add 79.8 ms · get 75.9 ms (150/150 OK, 11.7 s) |

Server-side latency from CloudWatch: GetItem ≈ 1.2–1.8 ms, PutItem ≈ 3.4–3.5 ms. The database is under 5% of the ~65–80 ms the
client measures; the rest is the network path laptop → ALB → ECS.

## Eventual consistency observations (Part 3)
| | eventual reads | strong reads |
|---|---|---|
| create → immediate get: stale first reads | 0 / 50 | 0 / 50 |
| add item → immediate get: stale first reads | 0 / 50 | 0 / 50 |
| 10 clients × 20 concurrent adds to one cart | 200 / 200, 0 lost | 200 / 200, 0 lost |

No stale read appeared even in eventual mode: each read arrives one ~65 ms round trip after the write, longer than DynamoDB normally
needs to update its replicas. (`max_ms_to_consistent` ≈ 155–165 ms is the slowest single GET, not a staleness window.) Zero lost
updates come from the atomic `if_not_exists(#i.#p, :zero) + :q` increment, not from the read mode.
User impact if a read were stale: the cart page would miss an item just added. Mitigation: `ConsistentRead` for the cart page
(read-your-writes), return the updated cart from writes, retry once on a 404 right after create.

## NoSQL vs SQL trade-offs discovered
- DynamoDB serves only the access patterns designed up front (by `cart_id`, by customer via the GSI); a new question such as "which
  carts contain product 7" needs a new index or a full scan, while SQL answers it with a query.
- A single-item conditional update gives the atomicity MySQL needed a transaction for; multi-item atomicity would need
  `TransactWriteItems`.
- No connection pool to size or exhaust; on-demand billing charges per request rather than per running instance.
- Consistency is a per-read choice with a price (strong = 2× read units), not a fixed property of the database.

## Learning notes (Anuj)
- **Surprised me:** DynamoDB answered in 1–4 ms, so almost all measured latency was network; eventual reads never returned stale
  data at this scale.
- **Failed initially, Part 1 (Sushant):** `items` reserved word (`#i`), route-table lookup timing (`depends_on`), Windows Smart App
  Control (Terraform in Docker).
- **Failed initially, Part 2:** (1) building on an Apple Silicon Mac labelled the runtime image arm64 → pinned
  `FROM --platform=linux/amd64` on the runtime stage (no effect on x86 builds); (2) Terraform's Docker provider hung for 20 minutes at
  `docker_image.app` with Docker Desktop → built and pushed with `docker buildx --platform linux/amd64 --push` and applied
  `-target=module.ecs` with a temporary local edit (not committed); (3) running `go mod tidy` before `dynamo.go` existed would have
  removed the new SDK dependencies → ran it afterwards.
- **Design validation:** CLI checks (Part 1), smoke tests, the 150-op test, consistency tests in both read modes, Contributor Insights.
