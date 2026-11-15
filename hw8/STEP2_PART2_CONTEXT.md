# Handoff: STEP II Part 2 (DynamoDB API) — from Sushant (STEP II Part 1)

## What is done (branch `dynamodb-part1`, new files only, MySQL code untouched)
| File | What |
|---|---|
| `hw8/terraform/dynamodb.tf` | Adds `module "dynamodb"`, finds the private route table, outputs `dynamodb_table_name` |
| `hw8/terraform/modules/dynamodb/` | Table `hw8-shopping-cart-carts` (key `cart_id` N, on-demand, GSI `customer_id-index`, TTL `expires_at`) + DynamoDB VPC gateway endpoint |
| `hw8/DYNAMODB_DESIGN.md` | Design decisions, item format, **contract for Part 2**, MySQL comparison, validation results, what failed initially |
| `hw8/screenshots/p1-*.png` | Table design + validated operations (atomic add, duplicate ID, GSI, 404 condition) |
Validated on AWS (us-east-1): every operation your endpoints need works from the CLI (see the design doc).

## Start
```
git fetch origin
git checkout -b dynamodb-part2 origin/dynamodb-part1
```

## What Part 2 has to do
1. **Go code** in `hw8/src/` (suggested new file `dynamo.go`; keep `carts.go`/`db.go` as they are):
   - `main()`: if `os.Getenv("CART_STORE") == "dynamodb"` → create the DynamoDB client and register the DynamoDB handlers on the
     **same three routes**; otherwise keep the current MySQL path (`openDB()`, `createSchema()`, existing handlers).
   - Reuse `parseCartID`, `respondError`, `respondInternalError`, `cart`, `cartItem`, `createCartRequest` from `carts.go`, so
     validation, JSON and error bodies are **identical** to MySQL (required for a fair comparison).
   - `go get github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/dynamodb` then `go mod tidy`.
2. **Terraform wiring** (the only edit to an existing file; agree with Patrick): in `main.tf`, module `ecs` → `environment`, add
   `{ name = "CART_STORE", value = var.cart_store }`, `{ name = "DYNAMODB_TABLE", value = module.dynamodb.table_name }`,
   `{ name = "AWS_REGION", value = var.aws_region }`, plus a `cart_store` variable (`"mysql"` default, `"dynamodb"`).
   LabRole already allows DynamoDB.
3. **Deploy:** `terraform apply -var="cart_store=dynamodb"` (or `-replace` the image as in the README after code changes).

## Contract (exact calls; validated)
| Endpoint | Call | Details | Errors |
|---|---|---|---|
| `POST /shopping-carts` | `PutItem` | `cart_id` = random int32 in [1, 2³¹−1], `customer_id`, `items` = empty map `M{}`, `created_at` (ISO), `expires_at` (now + 7 days, Unix s). `ConditionExpression: attribute_not_exists(cart_id)`; on `ConditionalCheckFailedException` pick a new ID and retry (≤ 3×) | 400 bad body · 500 |
| `GET /shopping-carts/{id}` | `GetItem` | Key `cart_id`. Eventually consistent by default; `ConsistentRead: true` behind a switch (e.g. env `DYNAMODB_CONSISTENT_READ=true`) for the consistency tests | 400 · **404 if no `Item`** · 500 |
| `POST /shopping-carts/{id}/items` | `UpdateItem` | `SET #i.#p = if_not_exists(#i.#p, :zero) + :q, expires_at = :exp` · `ConditionExpression: attribute_exists(cart_id)` · names `{"#i": "items", "#p": "<product_id>"}` | 400 · **404 on `ConditionalCheckFailedException`** · 500 · success = **204** (same as MySQL) |
⚠️ `items` is a **reserved word**: it must be `#i` in every expression (this failed in my first test).
Response: convert the `items` map (`"7": 5`) to `[{"product_id": 7, "quantity": 5}]`; empty map → `"items": []`.

```go
// sketch (aws-sdk-go-v2)
var ccf *types.ConditionalCheckFailedException
_, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
    TableName:           aws.String(table),
    Key:                 map[string]types.AttributeValue{"cart_id": &types.AttributeValueMemberN{Value: strconv.Itoa(int(id))}},
    UpdateExpression:    aws.String("SET #i.#p = if_not_exists(#i.#p, :zero) + :q, expires_at = :exp"),
    ConditionExpression: aws.String("attribute_exists(cart_id)"),
    ExpressionAttributeNames:  map[string]string{"#i": "items", "#p": strconv.Itoa(int(item.ProductID))},
    ExpressionAttributeValues: map[string]types.AttributeValue{
        ":zero": &types.AttributeValueMemberN{Value: "0"},
        ":q":    &types.AttributeValueMemberN{Value: strconv.Itoa(int(item.Quantity))},
        ":exp":  &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().Add(7*24*time.Hour).Unix(), 10)},
    },
})
if errors.As(err, &ccf) { /* 404 NOT_FOUND */ }
```

## Gotchas we already hit
- **Region:** the team Terraform uses **us-east-1** (`var.aws_region`); add `--region us-east-1` to AWS CLI commands if your default differs.
- **Windows 11 Smart App Control** may block Terraform's Docker provider ("An Application Control policy has blocked this file").
  Run Terraform in Docker instead (PowerShell, from `hw8/terraform`):
  ```powershell
  function tf { docker run --rm -v "${PWD}:/work" -w /work -v "$HOME\.aws:/root/.aws:ro" -v //var/run/docker.sock:/var/run/docker.sock -e TF_VAR_db_password=<password> hashicorp/terraform:1.9 @args }
  tf init; tf apply
  ```
- Learner Lab credentials expire with the session: refresh `~/.aws/credentials` if you see `InvalidClientTokenId`.
- Always `terraform destroy` after testing (NAT gateway, RDS and ALB cost money).

## Still open in STEP II (agree who does what)
Part 3 consistency tests (create→get, add→get, rapid concurrent updates) · Part 4 the **identical** 150-operation test →
`dynamodb_test_results.json` (same script/format as `mysql_test_results.json`) · Part 6 CloudWatch (latency, consumed capacity,
throttles, errors) · learning notes.
