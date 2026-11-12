# Wire together focused modules: network, alb, ecr, logging, database, ecs.
#   Internet -> ALB (public subnets) -> Shopping Cart service on ECS Fargate (private subnets)
#            -> RDS MySQL (private subnets)

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

module "alb" {
  source            = "./modules/alb"
  service_name      = var.service_name
  container_port    = var.container_port
  vpc_id            = module.network.vpc_id
  subnet_ids        = module.network.public_subnet_ids
  security_group_id = module.network.alb_security_group_id
  health_check_path = "/health"
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# MySQL in the private subnets, reachable only from the ECS tasks
module "database" {
  source            = "./modules/database"
  service_name      = var.service_name
  subnet_ids        = module.network.private_subnet_ids
  security_group_id = module.network.rds_security_group_id
  db_name           = var.db_name
  username          = var.db_username
  password          = var.db_password
}

# Reuse an existing IAM role for ECS tasks
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  subnet_ids         = module.network.private_subnet_ids
  security_group_ids = [module.network.ecs_security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  target_group_arn   = module.alb.target_group_arn
  region             = var.aws_region
  cpu                = var.cpu
  memory             = var.memory
  ecs_count          = var.ecs_count

  # The service opens its MySQL connection pool with these at startup
  environment = [
    { name = "DB_ADDR", value = module.database.endpoint },
    { name = "DB_USER", value = var.db_username },
    { name = "DB_PASSWORD", value = var.db_password },
    { name = "DB_NAME", value = var.db_name },
  ]

  # Wait for the image push, the ALB listener, the NAT route (the task pulls the image through it)
  # and the database (the task connects to it at startup)
  depends_on = [docker_registry_image.app, module.alb, module.network, module.database]
}


// Build & push the Go app image into ECR
resource "docker_image" "app" {
  # Use the URL from the ecr module, and tag it "latest"
  name = "${module.ecr.repository_url}:latest"

  build {
    # relative path from terraform/ → src/
    context = "../src"
    # Dockerfile defaults to "Dockerfile" in that context
  }
}

resource "docker_registry_image" "app" {
  # this will push :latest → ECR
  name = docker_image.app.name
}
