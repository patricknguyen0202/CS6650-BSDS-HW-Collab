# STEP II, Part 1: DynamoDB table for shopping carts (same API as the MySQL version).
#
#   One table, one item per cart:
#     cart_id      N  partition key (random positive int32, hashed by DynamoDB -> even spread)
#     customer_id  N  GSI partition key (customer history)
#     items        M  map product_id (as string) -> quantity (N), embedded, max 50 entries
#     created_at   S  ISO-8601 timestamp
#     expires_at   N  Unix seconds, TTL attribute (abandoned carts expire automatically)
#
# No sort key: every access is "the whole cart by cart_id".
# On-demand billing: no capacity planning, pay per request, absorbs spikes.

resource "aws_dynamodb_table" "carts" {
  name         = "${var.service_name}-carts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "cart_id"

  # Only key attributes are declared; items, created_at and expires_at are schemaless.
  attribute {
    name = "cart_id"
    type = "N"
  }

  attribute {
    name = "customer_id"
    type = "N"
  }

  # Customer history ("all carts of a customer"), the same access pattern MySQL serves
  # with idx_customer_id. KEYS_ONLY keeps the index small: it returns cart_id + customer_id.
  global_secondary_index {
    name            = "customer_id-index"
    hash_key        = "customer_id"
    projection_type = "KEYS_ONLY"
  }

  # Shopping carts are session data: DynamoDB deletes an item some time after expires_at
  # (free, no write capacity used).
  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  deletion_protection_enabled = false

  tags = { Name = "${var.service_name}-carts" }
}

# Gateway endpoint: tasks in the private subnets reach DynamoDB inside the AWS network
# instead of through the NAT gateway (lower latency, no NAT data charge, free).
resource "aws_vpc_endpoint" "dynamodb" {
  vpc_id            = var.vpc_id
  service_name      = "com.amazonaws.${var.region}.dynamodb"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = var.route_table_ids

  tags = { Name = "${var.service_name}-dynamodb-endpoint" }
}
