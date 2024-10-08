# staging.tfvars
aws_region         = "us-east-1"
project_name       = "myproject"
vpc_cidr           = "10.0.0.0/16"
public_subnet_cidr = "10.0.1.0/24"
private_subnet_cidr = "10.0.2.0/24"
rabbitmq_image     = "rabbitmq:3-management"
dispatcher_cpu     = "256"
dispatcher_memory  = "512"
rabbitmq_cpu       = "256"
rabbitmq_memory    = "512"
dispatcher_count   = 1
rabbitmq_count     = 1

# Note: dispatcher_image and dispatcher_image_tag are not set here 
# as they didn't have default values in the original configuration
# terraform apply -var-file=staging.tfvars -var="dispatcher_image=your_image" -var="dispatcher_image_tag=your_tag"