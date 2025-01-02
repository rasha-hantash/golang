run `docker compose up --build`
(you might need to go to your docker desktop UI and run the api containers)

run `go run dispatcher/dispatcher.go` 

run
```
curl -v http://localhost:8080/transaction \
-H "Content-Type: application/json" \
-d '{"txn_hash":"0x1234567890abcdef","from":"0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f","to":"0x8A791620dd6260079BF849Dc5567aDC3F2FdC318","value":1000000}'
```

todo: 
Security Considerations:

Use TLS for RabbitMQ connections.
Implement user authentication for RabbitMQ.
Consider using AWS PrivateLink or VPN for more secure connections.

 `docker run -e RABBITMQ_HOST=<aws-rabbitmq-public-dns> -e RABBITMQ_PORT=5672 your-docker-registry/operator:latest` 

 ```
 terraform init 
 terraform apply
 ```
 
todo 
- look into tls certifications with rabbit mq 
- todo look more into how i would deploy this via tf 
- todo look into jsut deploying this based on new docker image 

