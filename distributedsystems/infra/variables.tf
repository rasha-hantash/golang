# variables.tf
variable "aws_region" {
  description = "AWS region"
}

variable "project_name" {
  description = "Name of the project"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
}

variable "public_subnet_cidr" {
  description = "CIDR block for the public subnet"
}

variable "private_subnet_cidr" {
  description = "CIDR block for the private subnet"
}

variable "dispatcher_cpu" {
  description = "CPU units for Dispatcher task"
}

variable "dispatcher_memory" {
  description = "Memory for Dispatcher task"
}

variable "rabbitmq_cpu" {
  description = "CPU units for RabbitMQ task"
}

variable "rabbitmq_memory" {
  description = "Memory for RabbitMQ task"
}

variable "dispatcher_count" {
  description = "Number of Dispatcher tasks to run"
}

variable "rabbitmq_count" {
  description = "Number of RabbitMQ tasks to run"
}

variable "my_ip" {
  description = "Rasha's IP address"
  type        = string
}

variable "dispatcher_image" {
  description = "Docker image for the Dispatcher"
}

variable "dispatcher_image_tag" {
  description = "Tag for the Dispatcher Docker image"
}

variable "rabbitmq_image" {
  description = "Docker image for RabbitMQ"
}

variable "rabbitmq_image_tag" {
  description = "Tag for the RabbitMQ Docker image"
}