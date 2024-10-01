run `docker compose up --build`
(you might need to go to your docker desktop UI and run the api containers)

run `go run dispatcher/dispatcher.go` 

run
```
curl -v http://localhost:8080/transaction \
-H "Content-Type: application/json" \
-d '{"txn_hash":"0x1234567890abcdef","from":"0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f","to":"0x8A791620dd6260079BF849Dc5567aDC3F2FdC318","value":1000000}'
```

