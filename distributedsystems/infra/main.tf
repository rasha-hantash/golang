# Provider configuration
provider "aws" {
  region = var.aws_region
}

# VPC
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true
  
  tags = {
    Name = "${var.project_name}-vpc"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project_name}-igw"
  }
}

# Public subnet
resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = var.public_subnet_cidr
  availability_zone       = "${var.aws_region}a"
  map_public_ip_on_launch = true
  
  tags = {
    Name = "${var.project_name}-public-subnet"
  }
}

# Private subnet
resource "aws_subnet" "private" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnet_cidr
  availability_zone = "${var.aws_region}b"
  
  tags = {
    Name = "${var.project_name}-private-subnet"
  }
}

# Route table for public subnet
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.project_name}-public-rt"
  }
}

# Route table for private subnet
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project_name}-private-rt"
  }
}

# Associate public subnet with public route table
resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# Associate private subnet with private route table
resource "aws_route_table_association" "private" {
  subnet_id      = aws_subnet.private.id
  route_table_id = aws_route_table.private.id
}

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"
}

# ECS Task Definition for Dispatcher
resource "aws_ecs_task_definition" "dispatcher" {
  family                   = "${var.project_name}-dispatcher"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.dispatcher_cpu
  memory                   = var.dispatcher_memory

  container_definitions = jsonencode([
    {
      name  = "dispatcher"
      image = "${var.dispatcher_image}:${var.dispatcher_image_tag}"
      portMappings = [
        {
          containerPort = 80
          hostPort      = 80
        }
      ]
    }
  ])
}

# ECS Task Definition for RabbitMQ
resource "aws_ecs_task_definition" "rabbitmq" {
  family                   = "${var.project_name}-rabbitmq"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.rabbitmq_cpu
  memory                   = var.rabbitmq_memory

  container_definitions = jsonencode([
    {
      name  = "rabbitmq"
      image = var.rabbitmq_image
      portMappings = [
        {
          containerPort = 5672
          hostPort      = 5672
        },
        {
          containerPort = 15672
          hostPort      = 15672
        }
      ]
    }
  ])
}

# ECS Service for Dispatcher
resource "aws_ecs_service" "dispatcher" {
  name            = "${var.project_name}-dispatcher-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.dispatcher.arn
  launch_type     = "FARGATE"
  desired_count   = var.dispatcher_count

  network_configuration {
    subnets         = [aws_subnet.private.id]
    security_groups = [aws_security_group.dispatcher.id]
  }
}

# ECS Service for RabbitMQ
resource "aws_ecs_service" "rabbitmq" {
  name            = "${var.project_name}-rabbitmq-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.rabbitmq.arn
  launch_type     = "FARGATE"
  desired_count   = var.rabbitmq_count

  network_configuration {
    subnets          = [aws_subnet.public.id]
    security_groups  = [aws_security_group.rabbitmq.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.rabbitmq.arn
    container_name   = "rabbitmq"
    container_port   = 5672
  }
}

# Application Load Balancer
resource "aws_lb" "rabbitmq" {
  name               = "${var.project_name}-rabbitmq-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = [aws_subnet.public.id]

  enable_deletion_protection = false
}

# ALB Listener
resource "aws_lb_listener" "rabbitmq" {
  load_balancer_arn = aws_lb.rabbitmq.arn
  port              = "5672"
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.rabbitmq.arn
  }
}

# ALB Target Group
resource "aws_lb_target_group" "rabbitmq" {
  name        = "${var.project_name}-rabbitmq-tg"
  port        = 5672
  protocol    = "TCP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    healthy_threshold   = "3"
    interval            = "30"
    protocol            = "TCP"
    unhealthy_threshold = "3"
  }
}

# Security Group for Dispatcher
resource "aws_security_group" "dispatcher" {
  name        = "${var.project_name}-dispatcher-sg"
  description = "Security group for Dispatcher ECS tasks"
  vpc_id      = aws_vpc.main.id

  egress {
    from_port       = 5672
    to_port         = 5672
    protocol        = "tcp"
    security_groups = [aws_security_group.rabbitmq.id]
  }
}

# Security Group for RabbitMQ
resource "aws_security_group" "rabbitmq" {
  name        = "${var.project_name}-rabbitmq-sg"
  description = "Security group for RabbitMQ ECS tasks"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 5672
    to_port         = 5672
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id, aws_security_group.dispatcher.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# Security Group for ALB
resource "aws_security_group" "alb" {
  name        = "${var.project_name}-alb-sg"
  description = "Security group for RabbitMQ ALB"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port   = 5672
    to_port     = 5672
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]  # Consider restricting this to known IP ranges
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# # Provider configuration
# provider "aws" {
#   region = var.aws_region
# }

# # VPC
# resource "aws_vpc" "main" {
#   cidr_block           = var.vpc_cidr
#   enable_dns_hostnames = true
#   enable_dns_support   = true
  
#   tags = {
#     Name = "${var.project_name}-vpc"
#   }
# }

# # Internet Gateway
# resource "aws_internet_gateway" "main" {
#   vpc_id = aws_vpc.main.id

#   tags = {
#     Name = "${var.project_name}-igw"
#   }
# }

# # Public subnets
# resource "aws_subnet" "public1" {
#   vpc_id                  = aws_vpc.main.id
#   cidr_block              = var.public_subnet_cidr1
#   availability_zone       = "${var.aws_region}a"
#   map_public_ip_on_launch = true
  
#   tags = {
#     Name = "${var.project_name}-public-subnet-1"
#   }
# }

# resource "aws_subnet" "public2" {
#   vpc_id                  = aws_vpc.main.id
#   cidr_block              = var.public_subnet_cidr2
#   availability_zone       = "${var.aws_region}b"
#   map_public_ip_on_launch = true
  
#   tags = {
#     Name = "${var.project_name}-public-subnet-2"
#   }
# }

# # Private subnet
# resource "aws_subnet" "private" {
#   vpc_id            = aws_vpc.main.id
#   cidr_block        = var.private_subnet_cidr
#   availability_zone = "${var.aws_region}c"
  
#   tags = {
#     Name = "${var.project_name}-private-subnet"
#   }
# }

# # Route table for public subnets
# resource "aws_route_table" "public" {
#   vpc_id = aws_vpc.main.id

#   route {
#     cidr_block = "0.0.0.0/0"
#     gateway_id = aws_internet_gateway.main.id
#   }

#   tags = {
#     Name = "${var.project_name}-public-rt"
#   }
# }

# # Route table for private subnet
# resource "aws_route_table" "private" {
#   vpc_id = aws_vpc.main.id

#   tags = {
#     Name = "${var.project_name}-private-rt"
#   }
# }

# # Associate public subnets with public route table
# resource "aws_route_table_association" "public1" {
#   subnet_id      = aws_subnet.public1.id
#   route_table_id = aws_route_table.public.id
# }

# resource "aws_route_table_association" "public2" {
#   subnet_id      = aws_subnet.public2.id
#   route_table_id = aws_route_table.public.id
# }

# # Associate private subnet with private route table
# resource "aws_route_table_association" "private" {
#   subnet_id      = aws_subnet.private.id
#   route_table_id = aws_route_table.private.id
# }

# # ECS Cluster
# resource "aws_ecs_cluster" "main" {
#   name = "${var.project_name}-cluster"
# }

# # ECS Task Definition for Dispatcher
# resource "aws_ecs_task_definition" "dispatcher" {
#   family                   = "${var.project_name}-dispatcher"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = var.dispatcher_cpu
#   memory                   = var.dispatcher_memory

#   container_definitions = jsonencode([
#     {
#       name  = "dispatcher"
#       image = "${var.dispatcher_image}:${var.dispatcher_image_tag}"
#       portMappings = [
#         {
#           containerPort = 80
#           hostPort      = 80
#         }
#       ]
#     }
#   ])
# }

# # ECS Task Definition for RabbitMQ
# resource "aws_ecs_task_definition" "rabbitmq" {
#   family                   = "${var.project_name}-rabbitmq"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = var.rabbitmq_cpu
#   memory                   = var.rabbitmq_memory

#   container_definitions = jsonencode([
#     {
#       name  = "rabbitmq"
#       image = var.rabbitmq_image
#       portMappings = [
#         {
#           containerPort = 5672
#           hostPort      = 5672
#         },
#         {
#           containerPort = 15672
#           hostPort      = 15672
#         }
#       ]
#     }
#   ])
# }

# # ECS Service for Dispatcher
# resource "aws_ecs_service" "dispatcher" {
#   name            = "${var.project_name}-dispatcher-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.dispatcher.arn
#   launch_type     = "FARGATE"
#   desired_count   = var.dispatcher_count

#   network_configuration {
#     subnets         = [aws_subnet.private.id]
#     security_groups = [aws_security_group.dispatcher.id]
#   }
# }

# # ECS Service for RabbitMQ
# resource "aws_ecs_service" "rabbitmq" {
#   name            = "${var.project_name}-rabbitmq-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.rabbitmq.arn
#   launch_type     = "FARGATE"
#   desired_count   = var.rabbitmq_count

#   network_configuration {
#     subnets          = [aws_subnet.public1.id, aws_subnet.public2.id]
#     security_groups  = [aws_security_group.rabbitmq.id]
#     assign_public_ip = true
#   }

#   load_balancer {
#     target_group_arn = aws_lb_target_group.rabbitmq.arn
#     container_name   = "rabbitmq"
#     container_port   = 5672
#   }
# }

# # Application Load Balancer
# resource "aws_lb" "rabbitmq" {
#   name               = "${var.project_name}-rabbitmq-alb"
#   internal           = false
#   load_balancer_type = "application"
#   security_groups    = [aws_security_group.alb.id]
#   subnets            = [aws_subnet.public1.id, aws_subnet.public2.id]

#   enable_deletion_protection = false
# }

# # ALB Listener
# resource "aws_lb_listener" "rabbitmq" {
#   load_balancer_arn = aws_lb.rabbitmq.arn
#   port              = "5672"
#   protocol          = "TCP"

#   default_action {
#     type             = "forward"
#     target_group_arn = aws_lb_target_group.rabbitmq.arn
#   }
# }

# # ALB Target Group
# resource "aws_lb_target_group" "rabbitmq" {
#   name        = "${var.project_name}-rabbitmq-tg"
#   port        = 5672
#   protocol    = "TCP"
#   vpc_id      = aws_vpc.main.id
#   target_type = "ip"

#   health_check {
#     healthy_threshold   = "3"
#     interval            = "30"
#     protocol            = "TCP"
#     unhealthy_threshold = "3"
#   }
# }

# # Security Group for Dispatcher
# resource "aws_security_group" "dispatcher" {
#   name        = "${var.project_name}-dispatcher-sg"
#   description = "Security group for Dispatcher ECS tasks"
#   vpc_id      = aws_vpc.main.id

#   egress {
#     from_port       = 5672
#     to_port         = 5672
#     protocol        = "tcp"
#     security_groups = [aws_security_group.rabbitmq.id]
#   }
# }

# # Security Group for RabbitMQ
# resource "aws_security_group" "rabbitmq" {
#   name        = "${var.project_name}-rabbitmq-sg"
#   description = "Security group for RabbitMQ ECS tasks"
#   vpc_id      = aws_vpc.main.id

#   ingress {
#     from_port       = 5672
#     to_port         = 5672
#     protocol        = "tcp"
#     security_groups = [aws_security_group.alb.id, aws_security_group.dispatcher.id]
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# # Security Group for ALB
# resource "aws_security_group" "alb" {
#   name        = "${var.project_name}-alb-sg"
#   description = "Security group for RabbitMQ ALB"
#   vpc_id      = aws_vpc.main.id

#   ingress {
#     from_port   = 5672
#     to_port     = 5672
#     protocol    = "tcp"
#     cidr_blocks = ["0.0.0.0/0"]  # Consider restricting this to known IP ranges
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# # Variables
# variable "aws_region" {
#   description = "AWS region"
#   default     = "us-west-2"
# }

# variable "project_name" {
#   description = "Name of the project"
#   default     = "myproject"
# }

# variable "vpc_cidr" {
#   description = "CIDR block for the VPC"
#   default     = "10.0.0.0/16"
# }

# variable "public_subnet_cidr1" {
#   description = "CIDR block for the first public subnet"
#   default     = "10.0.1.0/24"
# }

# variable "public_subnet_cidr2" {
#   description = "CIDR block for the second public subnet"
#   default     = "10.0.2.0/24"
# }

# variable "private_subnet_cidr" {
#   description = "CIDR block for the private subnet"
#   default     = "10.0.3.0/24"
# }

# variable "dispatcher_image" {
#   description = "Docker image for the Dispatcher"
# }

# variable "dispatcher_image_tag" {
#   description = "Tag for the Dispatcher Docker image"
# }

# variable "rabbitmq_image" {
#   description = "Docker image for RabbitMQ"
#   default     = "rabbitmq:3-management"
# }

# variable "dispatcher_cpu" {
#   description = "CPU units for Dispatcher task"
#   default     = "256"
# }

# variable "dispatcher_memory" {
#   description = "Memory for Dispatcher task"
#   default     = "512"
# }

# variable "rabbitmq_cpu" {
#   description = "CPU units for RabbitMQ task"
#   default     = "256"
# }

# variable "rabbitmq_memory" {
#   description = "Memory for RabbitMQ task"
#   default     = "512"
# }

# variable "dispatcher_count" {
#   description = "Number of Dispatcher tasks to run"
#   default     = 1
# }

# variable "rabbitmq_count" {
#   description = "Number of RabbitMQ tasks to run"
#   default     = 1
# }

# # Outputs
# output "vpc_id" {
#   value       = aws_vpc.main.id
#   description = "ID of the VPC"
# }

# output "public_subnet_ids" {
#   value       = [aws_subnet.public1.id, aws_subnet.public2.id]
#   description = "IDs of the public subnets"
# }

# output "private_subnet_id" {
#   value       = aws_subnet.private.id
#   description = "ID of the private subnet"
# }

# output "rabbitmq_service_name" {
#   value       = aws_ecs_service.rabbitmq.name
#   description = "Name of the RabbitMQ ECS service"
# }

# output "rabbitmq_task_definition" {
#   value       = aws_ecs_task_definition.rabbitmq.arn
#   description = "ARN of the RabbitMQ task definition"
# }

# output "dispatcher_service_name" {
#   value       = aws_ecs_service.dispatcher.name
#   description = "Name of the Dispatcher ECS service"
# }

# output "dispatcher_task_definition" {
#   value       = aws_ecs_task_definition.dispatcher.arn
#   description = "ARN of the Dispatcher task definition"
# }

# output "rabbitmq_endpoint" {
#   value       = aws_lb.rabbitmq.dns_name
#   description = "The DNS name of the RabbitMQ ALB"
# }

###### above terraform has 2 AZ for increased availability and fault tolerance 

# provider "aws" {
#   region = "us-east-1"  # or your preferred region
# }

# # Single ECR Repository for all services
# resource "aws_ecr_repository" "services" {
#   name = "microservices-repo"
# }

# # ECS Cluster
# resource "aws_ecs_cluster" "main" {
#   name = "dispatcher-rabbitmq-cluster"
# }

# # ECS Task Definitions
# resource "aws_ecs_task_definition" "dispatcher" {
#   family                   = "dispatcher"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = "256"
#   memory                   = "512"

#   container_definitions = jsonencode([
#     {
#       name  = "dispatcher"
#       image = "${aws_ecr_repository.services.repository_url}:dispatcher-latest"
#       portMappings = [
#         { containerPort = 50051 }
#       ]
#     }
#   ])
# }

# resource "aws_ecs_task_definition" "operator" {
#   family                   = "operator"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = "256"
#   memory                   = "512"

#   container_definitions = jsonencode([
#     {
#       name  = "operator"
#       image = "${aws_ecr_repository.services.repository_url}:operator-latest"
#       portMappings = [
#         { containerPort = 8080 }  # Adjust this port as needed
#       ]
#     }
#   ])
# }

# # Create a private key for RabbitMQ
# resource "tls_private_key" "rabbitmq_key" {
#   algorithm = "RSA"
# }

# # Create a self-signed certificate
# resource "tls_self_signed_cert" "rabbitmq_cert" {
#   private_key_pem = tls_private_key.rabbitmq_key.private_key_pem

#   subject {
#     common_name  = "rabbitmq.example.com"
#     organization = "Example Org"
#   }

#   validity_period_hours = 8760  # 1 year

#   allowed_uses = [
#     "key_encipherment",
#     "digital_signature",
#     "server_auth",
#   ]
# }

# # Store the certificate and key in AWS Secrets Manager
# resource "aws_secretsmanager_secret" "rabbitmq_tls" {
#   name = "rabbitmq-tls-secrets"
# }

# resource "aws_secretsmanager_secret_version" "rabbitmq_tls" {
#   secret_id = aws_secretsmanager_secret.rabbitmq_tls.id
#   secret_string = jsonencode({
#     "tls.crt" = tls_self_signed_cert.rabbitmq_cert.cert_pem
#     "tls.key" = tls_private_key.rabbitmq_key.private_key_pem
#   })
# }

# resource "aws_ecs_task_definition" "rabbitmq" {
#   family                   = "rabbitmq"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = "256"
#   memory                   = "512"

#   container_definitions = jsonencode([
#     {
#       name  = "rabbitmq"
#       image = "${aws_ecr_repository.services.repository_url}:rabbitmq-latest"
#       portMappings = [
#         { containerPort = 5672 },
#         { containerPort = 15672 }
#       ]
#       secrets = [
#         {
#           name      = "RABBITMQ_SSL_CERTFILE",
#           valueFrom = "${aws_secretsmanager_secret.rabbitmq_tls.arn}:tls.crt::"
#         },
#         {
#           name      = "RABBITMQ_SSL_KEYFILE",
#           valueFrom = "${aws_secretsmanager_secret.rabbitmq_tls.arn}:tls.key::"
#         }
#       ]
#       environment = [
#         { name = "RABBITMQ_SSL_CACERTFILE", value = "/etc/rabbitmq/ca_certificate.pem" },
#         { name = "RABBITMQ_SSL_VERIFY", value = "verify_peer" },
#         { name = "RABBITMQ_SSL_FAIL_IF_NO_PEER_CERT", value = "false" }
#       ]
#     }
#   ])
# }

# # ECS Services
# resource "aws_ecs_service" "dispatcher" {
#   name            = "dispatcher-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.dispatcher.arn
#   launch_type     = "FARGATE"
#   desired_count   = 1

#   network_configuration {
#     subnets          = data.aws_subnets.default.ids
#     assign_public_ip = true
#     security_groups  = [aws_security_group.dispatcher_sg.id]
#   }
# }

# resource "aws_ecs_service" "operator" {
#   name            = "operator-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.operator.arn
#   launch_type     = "FARGATE"
#   desired_count   = 1

#   network_configuration {
#     subnets          = data.aws_subnets.default.ids
#     assign_public_ip = true
#     security_groups  = [aws_security_group.operator_sg.id]
#   }
# }

# resource "aws_ecs_service" "rabbitmq" {
#   name            = "rabbitmq-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.rabbitmq.arn
#   launch_type     = "FARGATE"
#   desired_count   = 1

#   network_configuration {
#     subnets          = data.aws_subnets.default.ids
#     assign_public_ip = true
#     security_groups  = [aws_security_group.rabbitmq_sg.id]
#   }
# }

# # Security Groups
# resource "aws_security_group" "dispatcher_sg" {
#   name        = "dispatcher-sg"
#   description = "Security group for dispatcher"
#   vpc_id      = data.aws_vpc.default.id

#   ingress {
#     from_port   = 50051
#     to_port     = 50051
#     protocol    = "tcp"
#     cidr_blocks = [data.aws_vpc.default.cidr_block]
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# resource "aws_security_group" "operator_sg" {
#   name        = "operator-sg"
#   description = "Security group for operator"
#   vpc_id      = data.aws_vpc.default.id

#   ingress {
#     from_port   = 8080  # Adjust this port as needed
#     to_port     = 8080  # Adjust this port as needed
#     protocol    = "tcp"
#     cidr_blocks = [data.aws_vpc.default.cidr_block]
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# resource "aws_security_group" "rabbitmq_sg" {
#   name        = "rabbitmq-sg"
#   description = "Security group for RabbitMQ"
#   vpc_id      = data.aws_vpc.default.id

#   ingress {
#     from_port   = 5672
#     to_port     = 5672
#     protocol    = "tcp"
#     cidr_blocks = [data.aws_vpc.default.cidr_block]
#   }

#   ingress {
#     from_port   = 15672
#     to_port     = 15672
#     protocol    = "tcp"
#     cidr_blocks = [data.aws_vpc.default.cidr_block]
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# # Data sources to get default VPC and subnet information
# data "aws_vpc" "default" {
#   default = true
# }

# data "aws_subnets" "default" {
#   filter {
#     name   = "vpc-id"
#     values = [data.aws_vpc.default.id]
#   }
# }

#####################################

# provider "aws" {
#   region = "us-east-1"  # or your preferred region
# }

# # VPC
# resource "aws_vpc" "main" {
#   cidr_block           = "10.0.0.0/16"
#   enable_dns_hostnames = true
#   enable_dns_support   = true

#   tags = {
#     Name = "main-vpc"
#   }
# }

# # Subnets
# resource "aws_subnet" "private_1" {
#   vpc_id            = aws_vpc.main.id
#   cidr_block        = "10.0.1.0/24"
#   availability_zone = "us-east-1a"

#   tags = {
#     Name = "Private Subnet AZ A"
#   }
# }

# resource "aws_subnet" "private_2" {
#   vpc_id            = aws_vpc.main.id
#   cidr_block        = "10.0.2.0/24"
#   availability_zone = "us-east-1b"

#   tags = {
#     Name = "Private Subnet AZ B"
#   }
# }

# resource "aws_subnet" "public_1" {
#   vpc_id                  = aws_vpc.main.id
#   cidr_block              = "10.0.3.0/24"
#   availability_zone       = "us-east-1a"
#   map_public_ip_on_launch = true

#   tags = {
#     Name = "Public Subnet AZ A"
#   }
# }

# resource "aws_subnet" "public_2" {
#   vpc_id                  = aws_vpc.main.id
#   cidr_block              = "10.0.4.0/24"
#   availability_zone       = "us-east-1b"
#   map_public_ip_on_launch = true

#   tags = {
#     Name = "Public Subnet AZ B"
#   }
# }

# # Internet Gateway
# resource "aws_internet_gateway" "main" {
#   vpc_id = aws_vpc.main.id

#   tags = {
#     Name = "main-igw"
#   }
# }

# # Route Table
# resource "aws_route_table" "public" {
#   vpc_id = aws_vpc.main.id

#   route {
#     cidr_block = "0.0.0.0/0"
#     gateway_id = aws_internet_gateway.main.id
#   }

#   tags = {
#     Name = "public-route-table"
#   }
# }

# # Route Table Association
# resource "aws_route_table_association" "public_1" {
#   subnet_id      = aws_subnet.public_1.id
#   route_table_id = aws_route_table.public.id
# }

# resource "aws_route_table_association" "public_2" {
#   subnet_id      = aws_subnet.public_2.id
#   route_table_id = aws_route_table.public.id
# }

# # ECS Cluster
# resource "aws_ecs_cluster" "main" {
#   name = "dispatcher-rabbitmq-cluster"
# }

# # ECS Task Definitions
# resource "aws_ecs_task_definition" "dispatcher" {
#   family                   = "dispatcher"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = "256"
#   memory                   = "512"

#   container_definitions = jsonencode([
#     {
#       name  = "dispatcher"
#       image = "${aws_ecr_repository.dispatcher.repository_url}:latest"
#       portMappings = [
#         { containerPort = 50051 }
#       ]
#     }
#   ])
# }


# # Create a private key for RabbitMQ
# resource "tls_private_key" "rabbitmq_key" {
#   algorithm = "RSA"
# }

# # Create a self-signed certificate
# resource "tls_self_signed_cert" "rabbitmq_cert" {
#   private_key_pem = tls_private_key.rabbitmq_key.private_key_pem

#   subject {
#     common_name  = "rabbitmq.example.com"
#     organization = "Example Org"
#   }

#   validity_period_hours = 8760  # 1 year

#   allowed_uses = [
#     "key_encipherment",
#     "digital_signature",
#     "server_auth",
#   ]
# }

# # Store the certificate and key in AWS Secrets Manager
# resource "aws_secretsmanager_secret" "rabbitmq_tls" {
#   name = "rabbitmq-tls-secrets"
# }

# resource "aws_secretsmanager_secret_version" "rabbitmq_tls" {
#   secret_id = aws_secretsmanager_secret.rabbitmq_tls.id
#   secret_string = jsonencode({
#     "tls.crt" = tls_self_signed_cert.rabbitmq_cert.cert_pem
#     "tls.key" = tls_private_key.rabbitmq_key.private_key_pem
#   })
# }

# resource "aws_ecs_task_definition" "rabbitmq" {
#   family                   = "rabbitmq"
#   network_mode             = "awsvpc"
#   requires_compatibilities = ["FARGATE"]
#   cpu                      = "256"
#   memory                   = "512"

#   container_definitions = jsonencode([
#     {
#       name  = "rabbitmq"
#       image = "rabbitmq:3-management"
#       portMappings = [
#         { containerPort = 5672 },
#         { containerPort = 15672 }
#       ]
#        secrets = [
#         {
#           name      = "RABBITMQ_SSL_CERTFILE",
#           valueFrom = "${aws_secretsmanager_secret.rabbitmq_tls.arn}:tls.crt::"
#         },
#         {
#           name      = "RABBITMQ_SSL_KEYFILE",
#           valueFrom = "${aws_secretsmanager_secret.rabbitmq_tls.arn}:tls.key::"
#         }
#       ]
#       environment = [
#         { name = "RABBITMQ_SSL_CACERTFILE", value = "/etc/rabbitmq/ca_certificate.pem" },
#         { name = "RABBITMQ_SSL_VERIFY", value = "verify_peer" },
#         { name = "RABBITMQ_SSL_FAIL_IF_NO_PEER_CERT", value = "false" }
#       ]
#     }
#   ])
# }

# # ECS Services
# resource "aws_ecs_service" "dispatcher" {
#   name            = "dispatcher-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.dispatcher.arn
#   launch_type     = "FARGATE"
#   desired_count   = 1

#   network_configuration {
#     subnets          = [aws_subnet.private_1.id, aws_subnet.private_2.id]
#     assign_public_ip = false
#     security_groups  = [aws_security_group.dispatcher_sg.id]
#   }
# }

# resource "aws_ecs_service" "rabbitmq" {
#   name            = "rabbitmq-service"
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.rabbitmq.arn
#   launch_type     = "FARGATE"
#   desired_count   = 1

#   network_configuration {
#     subnets          = [aws_subnet.private_1.id, aws_subnet.private_2.id]
#     assign_public_ip = false
#     security_groups  = [aws_security_group.rabbitmq_sg.id]
#   }
# }

# # Security Groups
# resource "aws_security_group" "dispatcher_sg" {
#   name        = "dispatcher-sg"
#   description = "Security group for dispatcher"
#   vpc_id      = aws_vpc.main.id

#   ingress {
#     from_port   = 50051
#     to_port     = 50051
#     protocol    = "tcp"
#     cidr_blocks = [aws_vpc.main.cidr_block]
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# resource "aws_security_group" "rabbitmq_sg" {
#   name        = "rabbitmq-sg"
#   description = "Security group for RabbitMQ"
#   vpc_id      = aws_vpc.main.id

#   ingress {
#     from_port   = 5672
#     to_port     = 5672
#     protocol    = "tcp"
#     cidr_blocks = [aws_vpc.main.cidr_block]
#   }

#   ingress {
#     from_port   = 15672
#     to_port     = 15672
#     protocol    = "tcp"
#     cidr_blocks = [aws_vpc.main.cidr_block]
#   }

#   egress {
#     from_port   = 0
#     to_port     = 0
#     protocol    = "-1"
#     cidr_blocks = ["0.0.0.0/0"]
#   }
# }

# # ECR Repository
# resource "aws_ecr_repository" "dispatcher" {
#   name = "dispatcher-repo"
# }

# # provider "aws" {
# #   region = "your-region"
# # }

# # resource "aws_ecs_cluster" "my_cluster" {
# #   name = "my-app-cluster"
# # }

# # resource "aws_ecs_task_definition" "my_task" {
# #   family                   = "my-app-task"
# #   network_mode             = "awsvpc"
# #   requires_compatibilities = ["FARGATE"]
# #   cpu                      = "256"
# #   memory                   = "512"

# #   container_definitions = jsonencode([
# #     {
# #       name  = "rabbitmq"
# #       image = "rabbitmq:3-management"
# #       portMappings = [
# #         { containerPort = 5672 },
# #         { containerPort = 15672 }
# #       ]
# #     },
# #     {
# #       name  = "client"
# #       image = "${aws_ecr_repository.my_repo.repository_url}:client"
# #       portMappings = [
# #         { containerPort = 50051 }
# #       ]
# #       environment = [
# #         { name = "RABBITMQ_HOST", value = "localhost" },
# #         { name = "RABBITMQ_PORT", value = "5672" }
# #       ]
# #     }
# #   ])
# # }

# # resource "aws_ecs_service" "my_service" {
# #   name            = "my-app-service"
# #   cluster         = aws_ecs_cluster.my_cluster.id
# #   task_definition = aws_ecs_task_definition.my_task.arn
# #   launch_type     = "FARGATE"
# #   desired_count   = 1

# #   network_configuration {
# #     subnets          = ["subnet-xxxxxxxx", "subnet-yyyyyyyy"]
# #     assign_public_ip = true
# #   }
# # }

# # resource "aws_ecr_repository" "my_repo" {
# #   name = "my-app-repo"
# # }