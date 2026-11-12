variable "service_name" {
  type        = string
  description = "Base name for ECS resources"
}

variable "image" {
  type        = string
  description = "ECR image URI (with tag)"
}

variable "container_port" {
  type        = number
  description = "Port the app listens on"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Private subnets for the Fargate tasks"
}

variable "security_group_ids" {
  type        = list(string)
  description = "Security groups for the Fargate tasks"
}

variable "execution_role_arn" {
  type        = string
  description = "ECS task execution role ARN"
}

variable "task_role_arn" {
  type        = string
  description = "IAM role ARN for app permissions"
}

variable "log_group_name" {
  type        = string
  description = "CloudWatch log group name"
}

variable "ecs_count" {
  type        = number
  default     = 1
  description = "Desired Fargate task count"
}

variable "target_group_arn" {
  type        = string
  default     = null
  description = "ALB target group for the tasks (null = no load balancer)"
}

variable "environment" {
  type = list(object({
    name  = string
    value = string
  }))
  default     = []
  description = "Container environment variables"
}

variable "region" {
  type        = string
  description = "AWS region (for the awslogs driver)"
}

variable "cpu" {
  type        = string
  default     = "256"
  description = "CPU units"
}

variable "memory" {
  type        = string
  default     = "512"
  description = "Memory (MiB)"
}
