to run 
`docker compose up --build` 

note: if you run client.go locally and change the postgres hostname to `localhost` you will get a grpc context error saying the following "received context error while waiting for new LB policy update" error , this is likely because the ips are located within the docker network