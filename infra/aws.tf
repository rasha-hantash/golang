provider "aws" {
  region = "your-region"
}

resource "aws_ecs_cluster" "my_cluster" {
  name = "my-app-cluster"
}

resource "aws_ecs_task_definition" "my_task" {
  family                   = "my-app-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"

  container_definitions = jsonencode([
    {
      name  = "rabbitmq"
      image = "rabbitmq:3-management"
      portMappings = [
        { containerPort = 5672 },
        { containerPort = 15672 }
      ]
    },
    {
      name  = "client"
      image = "${aws_ecr_repository.my_repo.repository_url}:client"
      portMappings = [
        { containerPort = 50051 }
      ]
      environment = [
        { name = "RABBITMQ_HOST", value = "localhost" },
        { name = "RABBITMQ_PORT", value = "5672" }
      ]
    }
  ])
}

resource "aws_ecs_service" "my_service" {
  name            = "my-app-service"
  cluster         = aws_ecs_cluster.my_cluster.id
  task_definition = aws_ecs_task_definition.my_task.arn
  launch_type     = "FARGATE"
  desired_count   = 1

  network_configuration {
    subnets          = ["subnet-xxxxxxxx", "subnet-yyyyyyyy"]
    assign_public_ip = true
  }
}

resource "aws_ecr_repository" "my_repo" {
  name = "my-app-repo"
}