variable "service_name" {
  description = "Prefix for resource names"
  type        = string
}

variable "subnet_ids" {
  description = "Private subnets for the DB subnet group (at least 2 availability zones)"
  type        = list(string)
}

variable "security_group_id" {
  description = "Security group that allows MySQL from the ECS tasks only"
  type        = string
}

variable "db_name" {
  description = "Database created on the instance"
  type        = string
}

variable "username" {
  description = "Master username"
  type        = string
}

variable "password" {
  description = "Master password"
  type        = string
  sensitive   = true
}
