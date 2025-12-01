Distributed Ticketing System:

A final project from the college course "Distributed Systems"
The project consists of many completely individiual microservices entirely written in GO

To input Tickets:
docker compose exec etcd_service_1 etcdctl put "concert:58b85029-af94-498e-ae3a-2fda2b5d6c5a:available:basic" "10000"