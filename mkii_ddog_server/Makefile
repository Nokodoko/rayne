#TODO: fix EndpointWithPathParams log statement which will work for all endpoints using advanced routing, giving more detail to each failure point


all: build

build:
	go build .

c:
	curl localhost:8080/$(word 2,$(MAKECMDGOALS))

r:
	go run cmd/migrate/main.go

test:
	@go test -v ./...

build:
	@go build -o bin/fsDDServer cmd/main.go

events:
	curl localhost:8080/v1/events

down:
	curl localhost:8080/v1/downtimes

awsInt:
	curl localhost:8080/v1/integrations/aws

active:
	curl localhost:8080/v1/hosts/active

pull:
	curl localhost:8080/v1/pl/refresh/pull

stopper:
	curl localhost:8080/v1/pl/refresh/stop

pp:
	curl localhost:8080/v1/pl/refresh/pp
remove:
	curl localhost:8080/v1/pl/refresh/remove

test:
	go test ./services/user/

zip:
	rm ../mkii_ddog_server.zip
	echo "Removed prior zip file"
	zip -r ../mkii_ddog_server.zip ../mkii_ddog_server/
	echo "Made new zip file"

# down:
# 	@make r &
# 	zellij run --floating -- curl localhost:8080/downtimes
#


