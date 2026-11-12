variable "service_name" {
  description = "Used to name the ALB and target group"
  type        = string
}

variable "container_port" {
  description = "Port the tasks listen on"
  type        = number
}

variable "vpc_id" {
  description = "VPC of the target group"
  type        = string
}

variable "subnet_ids" {
  description = "Public subnets for the ALB"
  type        = list(string)
}

variable "security_group_id" {
  description = "Security group for the ALB"
  type        = string
}

variable "health_check_path" {
  description = "Path the target group checks on each task"
  type        = string
  default     = "/health"
}
