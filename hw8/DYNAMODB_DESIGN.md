# STEP II, Part 1: DynamoDB Table Design (Sushant Mudalgi)

Goal: store the same shopping carts as STEP I (MySQL) in DynamoDB, behind the **same API**
(`POST /shopping-carts`, `GET /shopping-carts/{id}`, `POST /shopping-carts/{id}/items`), so the two can be compared fairly.

## Access patterns (what the design must support)
| # | Access pattern | Endpoint | DynamoDB operation |
|---|---|---|---|
| 1 | Create a cart for a customer | `POST /shopping-carts` | `PutItem` (conditional) |
| 2 | Get one cart with all its items, by cart ID (< 50 ms) | `GET /shopping-carts/{id}` | `GetItem` on the partition key |
| 3 | Add / update an item in an existing cart | `POST /shopping-carts/{id}/items` | `UpdateItem` (atomic, conditional) |
| 4 | All carts of a customer (history; parity with MySQL's `idx_customer_id`) | none (not an endpoint) | `Query` on the GSI |

Every endpoint touches **exactly one cart**, by its ID. There are no joins, no multi-cart transactions and no range queries.

## Design decisions
| Decision | Choice | Why |
|---|---|---|
| **Table structure** | **One table**, `hw8-shopping-cart-carts`, **one item per cart** | Every access reads or writes one whole cart, so the cart is the natural unit. Nothing else needs its own table (no products/customers table in MySQL either). |
| **Partition key** | **`cart_id` (Number)** | Every operation knows the cart ID, so every read and write is a direct key lookup. DynamoDB **hashes** the partition key, so even sequential-looking IDs spread across partitions. Each cart is touched by only one shopper, so **no key is hot** under normal load. |
| **Sort key** | **None** | There is never a "range" inside a cart to query. Items live inside the cart item (below). |
| **Cart items** | **Embedded** map `items: { "<product_id>": <quantity> }` | A cart has at most 50 items of ~2 numbers each (well under 1 KB, far below the 400 KB item limit), and the API always returns the whole cart. One `GetItem` returns everything, which is the DynamoDB equivalent of MySQL's `LEFT JOIN`. A map keyed by product ID makes "add the same product again" one atomic update of one map entry. |
| **Cart ID generation** | **Random positive int32**, written with `attribute_not_exists(cart_id)`, retried on collision | DynamoDB has no AUTO_INCREMENT. A counter item would need an extra write per cart and becomes a **hot key** with millions of carts. Random IDs need no coordination; with 2³¹ IDs a collision is rare, and the condition makes it safe. The API still returns an `int32` `shopping_cart_id`, same as MySQL. |
| **Secondary index** | **GSI `customer_id-index`**, partition key `customer_id`, projection `KEYS_ONLY` | Supports customer history (pattern 4) like MySQL's `idx_customer_id`. KEYS_ONLY keeps it small: it returns cart IDs, then `GetItem`/`BatchGetItem` fetches carts if needed. GSIs are always eventually consistent. |
| **Expiration** | **TTL** on `expires_at` (Unix seconds, e.g. now + 7 days, refreshed on each update) | Carts are session data. DynamoDB deletes expired items for free, so abandoned carts don't pile up (MySQL would need a cleanup job). |
| **Capacity** | **On-demand** (`PAY_PER_REQUEST`) | No capacity planning, pays per request, absorbs flash-sale spikes; cost-efficient at homework scale. |
| **Network** | **VPC gateway endpoint** for DynamoDB | ECS runs in private subnets. The endpoint routes DynamoDB traffic inside AWS instead of through the NAT gateway: lower latency, no NAT charge, and fairer against MySQL (which is also inside the VPC). |

### Item format (what the API service stores)
```json
{
  "cart_id":     { "N": "1834201977" },
  "customer_id": { "N": "42" },
  "items":       { "M": { "7": { "N": "4" }, "8": { "N": "1" } } },
  "created_at":  { "S": "2026-11-10T18:00:00Z" },
  "expires_at":  { "N": "1795000000" }
}
```
Map keys must be strings, so `product_id` 7 is stored as `"7"` and converted back to an int32 in the response.
The API response is identical to MySQL's: `{"shopping_cart_id": 1834201977, "customer_id": 42, "items": [{"product_id": 7, "quantity": 4}, ...]}`.

## Contract for STEP II Part 2 (API implementation)
| Endpoint | Operation | Expression / condition | Errors |
|---|---|---|---|
| `POST /shopping-carts` | `PutItem` with `cart_id` = random int32 in [1, 2³¹−1], `customer_id`, `items` = empty map, `created_at`, `expires_at` | `ConditionExpression: attribute_not_exists(cart_id)` → on `ConditionalCheckFailedException`, pick a new ID and retry | 400 bad body, 500 DynamoDB error |
| `GET /shopping-carts/{id}` | `GetItem` key `cart_id` (eventually consistent by default; `ConsistentRead: true` for the consistency comparison) | none | 400 bad id, **404 if no item**, 500 |
| `POST /shopping-carts/{id}/items` | `UpdateItem` key `cart_id` | `UpdateExpression: SET items.#p = if_not_exists(items.#p, :zero) + :q, expires_at = :exp` · `ConditionExpression: attribute_exists(cart_id)` · `ExpressionAttributeNames: {"#p": "<product_id>"}` | 400 bad id/body, **404 on ConditionalCheckFailedException**, 500 |
Same validation and error bodies as STEP I (`parseCartID`, `respondError` in `carts.go`).
**Wiring for Part 2:** pass the table name to the service (e.g. add `DYNAMODB_TABLE = module.dynamodb.table_name` and `AWS_REGION`
to the ECS environment). Part 1 adds the table without changing any STEP I file.

**Why one `UpdateItem` (no transaction):** the condition (cart exists) and the increment happen **atomically on one item**, so two
concurrent adds of the same product both apply (no lost update), and a missing cart writes nothing. MySQL needed a transaction across
two tables for the same guarantee.

## Comparison with the MySQL design (STEP I)
| | MySQL (STEP I) | DynamoDB (STEP II) |
|---|---|---|
| Tables | 2: `shopping_carts`, `cart_items` | 1: `carts` |
| Cart ↔ item relationship | Foreign key `cart_items.cart_id` → `shopping_carts.id` (normalized) | Items **embedded** in the cart (denormalized) |
| Get cart | `LEFT JOIN` on two primary keys | One `GetItem` |
| Add item | Transaction: check cart + upsert row | One conditional `UpdateItem` |
| Cart ID | `AUTO_INCREMENT` | Random int32 + conditional put |
| Customer history | `idx_customer_id` (strongly consistent) | GSI `customer_id-index` (eventually consistent) |
| Integrity | Foreign key prevents orphaned items | No orphans possible: items exist only inside a cart |
| Consistency | ACID, read-your-writes | Eventually consistent reads by default; strong reads cost 2× |
| Scaling | Vertical (bigger instance), connection limit (pool of 10) | Horizontal, automatic partitioning, no connections |
| Flexible queries | Any SQL (e.g. "carts containing product 7") | Only the designed access patterns; new patterns need new indexes |

## Trade-offs
- **Embedding items** is fast for the whole-cart reads we need, but "which carts contain product X?" would require a scan. MySQL answers it with an index.
- **Random cart IDs** avoid a hot counter, but IDs are no longer ordered by creation time (use `created_at` if needed).
- **Eventually consistent reads** are half the cost but can briefly miss a just-written item (tested in Part 3).
- **GSI** costs extra writes on every cart create and is eventually consistent, acceptable for history.
- **Item size** grows with items; at 50 small items it's far below 400 KB, but unbounded carts would need item splitting (cart_id + sort key per item).

## Terraform (Part 1 adds only new files)
| File | What |
|---|---|
| `terraform/dynamodb.tf` | Calls the module, finds the private route table with a data source, outputs the table name |
| `terraform/modules/dynamodb/` | The table (key, GSI, TTL, on-demand) and the DynamoDB VPC gateway endpoint |
No STEP I (MySQL) file is modified.

## Validate the table design (before the API exists)
After `terraform apply -target=module.dynamodb`, from PowerShell (`$T = terraform output -raw dynamodb_table_name`):
```powershell
aws dynamodb describe-table --table-name $T --query "Table.{Key:KeySchema,GSI:GlobalSecondaryIndexes[].IndexName,Billing:BillingModeSummary.BillingMode}"
aws dynamodb describe-time-to-live --table-name $T
aws dynamodb put-item --table-name $T --item '{\"cart_id\":{\"N\":\"101\"},\"customer_id\":{\"N\":\"42\"},\"items\":{\"M\":{}}}' --condition-expression "attribute_not_exists(cart_id)"
aws dynamodb update-item --table-name $T --key '{\"cart_id\":{\"N\":\"101\"}}' --update-expression "SET items.#p = if_not_exists(items.#p, :z) + :q" --condition-expression "attribute_exists(cart_id)" --expression-attribute-names '{\"#p\":\"7\"}' --expression-attribute-values '{\":z\":{\"N\":\"0\"},\":q\":{\"N\":\"2\"}}'
aws dynamodb update-item --table-name $T --key '{\"cart_id\":{\"N\":\"101\"}}' --update-expression "SET items.#p = if_not_exists(items.#p, :z) + :q" --condition-expression "attribute_exists(cart_id)" --expression-attribute-names '{\"#p\":\"7\"}' --expression-attribute-values '{\":z\":{\"N\":\"0\"},\":q\":{\"N\":\"3\"}}'
aws dynamodb get-item --table-name $T --key '{\"cart_id\":{\"N\":\"101\"}}'
aws dynamodb query --table-name $T --index-name customer_id-index --key-condition-expression "customer_id = :c" --expression-attribute-values '{\":c\":{\"N\":\"42\"}}'
aws dynamodb update-item --table-name $T --key '{\"cart_id\":{\"N\":\"999\"}}' --update-expression "SET items.#p = :q" --condition-expression "attribute_exists(cart_id)" --expression-attribute-names '{\"#p\":\"7\"}' --expression-attribute-values '{\":q\":{\"N\":\"1\"}}'
aws dynamodb delete-item --table-name $T --key '{\"cart_id\":{\"N\":\"101\"}}'
```
Expected: key `cart_id` HASH, GSI `customer_id-index`, `PAY_PER_REQUEST`, TTL `ENABLED` on `expires_at`; after two adds, product `7` has
quantity **5** (2 + 3, atomic increment); the GSI query returns cart 101; the update on missing cart 999 fails with
**ConditionalCheckFailedException** (the API maps it to 404).
