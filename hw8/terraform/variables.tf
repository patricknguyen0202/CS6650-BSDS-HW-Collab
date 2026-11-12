# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-east-1"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "hw8-shopping-cart"
}

# Also the name prefix for every AWS resource (ALB name hw8-shopping-cart-alb stays under 32 characters)
variable "service_name" {
  type    = string
  default = "hw8-shopping-cart"
}

# Number of tasks behind the ALB. Set to 2 to show that any task can serve any cart
variable "ecs_count" {
  type    = number
  default = 1
}

variable "container_port" {
  type    = number
  default = 8080
}

# Standard ECS task settings from the assignment: 256 CPU units, 512 MB
variable "cpu" {
  type    = string
  default = "256"
}

variable "memory" {
  type    = string
  default = "512"
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}

# RDS MySQL settings
variable "db_name" {
  type    = string
  default = "shopping"
}

variable "db_username" {
  type    = string
  default = "admin"
}

# Set with TF_VAR_db_password, the same value on every apply.
# Must not contain "/", double quotes, "@" or spaces (RDS rejects them).
variable "db_password" {
  type      = string
  sensitive = true

  validation {
    condition     = length(var.db_password) >= 8 && length(var.db_password) <= 41
    error_message = "db_password must be 8 to 41 characters long (RDS MySQL limit)."
  }
}
