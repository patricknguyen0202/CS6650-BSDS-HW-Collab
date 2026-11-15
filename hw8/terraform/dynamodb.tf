# STEP II, Part 1 (Sushant): DynamoDB table for the shopping carts.
# This file only ADDS resources; it does not change any existing STEP I (MySQL) file.
# Terraform loads every .tf file in this folder, so `terraform apply` creates the table next to RDS.

# Route table of the private subnets, found from the first private subnet (no change to the network module).
# depends_on: read it only after the network module has created the route table and its subnet associations.
data "aws_route_table" "private" {
  subnet_id  = module.network.private_subnet_ids[0]
  depends_on = [module.network]
}

module "dynamodb" {
  source          = "./modules/dynamodb"
  service_name    = var.service_name
  region          = var.aws_region
  vpc_id          = module.network.vpc_id
  route_table_ids = [data.aws_route_table.private.id]
}

output "dynamodb_table_name" {
  description = "DynamoDB table for STEP II (pass to the service as DYNAMODB_TABLE in Part 2)"
  value       = module.dynamodb.table_name
}
