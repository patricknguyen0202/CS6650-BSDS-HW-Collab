# Public entry point in the public subnets (names max 32 chars)
resource "aws_lb" "this" {
  name               = "${var.service_name}-alb"
  load_balancer_type = "application"
  internal           = false
  subnets            = var.subnet_ids
  security_groups    = [var.security_group_id]

  # Idle timeout keeps the AWS default of 60 s: a request that waits longer than 60 s gets a 504.
}

# Target group: ECS registers each task's IP here
resource "aws_lb_target_group" "this" {
  name        = "${var.service_name}-tg"
  target_type = "ip" # required for Fargate
  protocol    = "HTTP"
  port        = var.container_port
  vpc_id      = var.vpc_id

  # Shorter than the 300 s default so redeploys do not hang
  deregistration_delay = 30

  health_check {
    path                = var.health_check_path
    matcher             = "200"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

# Forward HTTP on port 80 to the target group
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}
