# Replicated log

## Runbook
To start the cluster go to *build* directory and run `docker-compose up --build`.  
To write a message run `curl -X POST localhost:9000 -d '{"record": {"value": "some_value"}, "w": 3}'`.  
To get messages from master run `curl localhost:9000` and from secondaries port should be either **9001** or **9002**. 

## Iterations
  
### Iteration 1.
The Replicated Log should have the following deployment architecture: one Master and any number of Secondaries.
  
Master should expose simple HTTP server with: 
- POST method - appends a message into the in-memory list
- GET method - returns all messages from the in-memory list
  
Secondary should expose simple  HTTP server with:
- GET method - returns all replicated messages from the in-memory list
  
Properties and assumptions:
- after each POST request, the message should be replicated on every Secondary server
- Master should ensure that Secondaries have received a message via ACK
- POST request should be finished only after receiving ACKs from all Secondaries (blocking replication approach)
- to test that the replication is blocking, introduce the delay/sleep on the Secondary
- at this stage assume that the communication channel is a perfect link (no failures and messages lost)
- any RPC frameworks can be used for Master-Secondary communication (Sockets, language-specific RPC, HTTP, Rest, gRPC, …)
- Master and Secondaries should run in Docker
