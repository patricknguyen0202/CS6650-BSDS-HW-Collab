variable "service_name" {
  description = "Prefix for resource names"
  type        = string
}

variable "region" {
  description = "AWS region (for the VPC endpoint service name)"
  type        = string
}

variable "vpc_id" {
  description = "VPC of the ECS tasks"
  type        = string
}

variable "route_table_ids" {
  description = "Route tables of the private subnets (the gateway endpoint adds a DynamoDB route to them)"
  type        = list(string)
}
