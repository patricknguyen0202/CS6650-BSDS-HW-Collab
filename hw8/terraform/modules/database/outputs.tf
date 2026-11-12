output "endpoint" {
  description = "Connection endpoint (host:3306)"
  value       = aws_db_instance.this.endpoint
}
