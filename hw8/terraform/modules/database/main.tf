# RDS needs a subnet group that spans at least 2 availability zones. The private subnets do.
resource "aws_db_subnet_group" "this" {
  name       = "${var.service_name}-db-subnets"
  subnet_ids = var.subnet_ids
}

# MySQL 8.0 on the free tier class. Not reachable from the internet.
# skip_final_snapshot and deletion_protection = false let terraform destroy remove it at once.
resource "aws_db_instance" "this" {
  identifier             = "${var.service_name}-mysql"
  engine                 = "mysql"
  engine_version         = "8.0"
  instance_class         = "db.t3.micro"
  allocated_storage      = 20
  db_name                = var.db_name
  username               = var.username
  password               = var.password
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [var.security_group_id]
  publicly_accessible    = false
  skip_final_snapshot    = true
  deletion_protection    = false
}
