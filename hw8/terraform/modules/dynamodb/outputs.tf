output "table_name" {
  description = "DynamoDB table holding the shopping carts"
  value       = aws_dynamodb_table.carts.name
}

output "table_arn" {
  value = aws_dynamodb_table.carts.arn
}

output "customer_index_name" {
  value = "customer_id-index"
}
