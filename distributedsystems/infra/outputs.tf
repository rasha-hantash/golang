# Outputs
output "vpc_id" {
  value       = aws_vpc.main.id
  description = "ID of the VPC"
}

output "public_subnet_id" {
  value       = aws_subnet.public.id
  description = "ID of the public subnet"
}

output "private_subnet_id" {
  value       = aws_subnet.private.id
  description = "ID of the private subnet"
}

output "rabbitmq_service_name" {
  value       = aws_ecs_service.rabbitmq.name
  description = "Name of the RabbitMQ ECS service"
}

output "rabbitmq_task_definition" {
  value       = aws_ecs_task_definition.rabbitmq.arn
  description = "ARN of the RabbitMQ task definition"
}

output "dispatcher_service_name" {
  value       = aws_ecs_service.dispatcher.name
  description = "Name of the Dispatcher ECS service"
}

output "dispatcher_task_definition" {
  value       = aws_ecs_task_definition.dispatcher.arn
  description = "ARN of the Dispatcher task definition"
}

output "rabbitmq_endpoint" {
  value       = aws_lb.rabbitmq.dns_name
  description = "The DNS name of the RabbitMQ ALB"
}
