output "alb_url" {
  description = "Base URL of the Shopping Cart service"
  value       = "http://${module.alb.dns_name}"
}

output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

output "rds_endpoint" {
  description = "RDS MySQL endpoint (host:3306), reachable only from the ECS tasks"
  value       = module.database.endpoint
}
