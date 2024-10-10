terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.50.0"
    }
  }
}

locals {
  environment = terraform.workspace
}

provider "aws" {
  region = var.aws_region
}

# Create ECR repository
resource "aws_ecr_repository" "platform" {
  name                 = "${local.environment}-platform"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

# Create AWS Secrets Manager secret for dispatcher config
resource "aws_secretsmanager_secret" "dispatcher_config" {
  name = "${local.environment}-dispatcher-config"

  tags = {
    Environment = local.environment
    Service     = "dispatcher"
  }
}

resource "aws_secretsmanager_secret_version" "dispatcher_config_version" {
  secret_id     = aws_secretsmanager_secret.dispatcher_config.id
  secret_string = jsonencode({}) # Empty JSON object
}

# Create AWS Secrets Manager secret for operator config
resource "aws_secretsmanager_secret" "operator_config" {
  name = "${local.environment}-operator-config"
  tags = {
    Environment = local.environment
    Service     = "operator"
  }
}

resource "aws_secretsmanager_secret_version" "operator_config_version" {
  secret_id     = aws_secretsmanager_secret.operator_config.id
  secret_string = jsonencode({}) # Empty JSON object
}


# Outputs
output "ecr_repository_url" {
  value       = aws_ecr_repository.platform.repository_url
  description = "The URL of the ECR repository"
}

output "dispatcher_secret_arn" {
  value       = aws_secretsmanager_secret.dispatcher_config.arn
  description = "The ARN of the Secrets Manager secret"
}

output "operator_secret_arn" {
  value       = aws_secretsmanager_secret.operator_config.arn
  description = "The ARN of the Secrets Manager secret"
}

# Variables (define these in a separate variables.tf file)
variable "aws_region" {
  description = "The AWS region to create resources in"
  default     = "us-east-1"
}
