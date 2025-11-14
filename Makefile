GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	#the `find.exe` is different from `find` in bash/shell.
	#to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
	#changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
	#Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	INTERNAL_PROTO_FILES=$(shell $(Git_Bash) -c "find internal -name *.proto")
	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
else
	INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
	API_PROTO_FILES=$(shell find api -name *.proto)
endif

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: config
# generate internal proto
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: api
# generate api proto
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./api \
 	       --go-http_out=paths=source_relative:./api \
 	       --go-grpc_out=paths=source_relative:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: proto
# generate proto code (buf + protoc)
proto:
	make api;
	make config;

.PHONY: proto-clean
# clean generated proto files
proto-clean:
	@echo "Cleaning generated proto files..."
	@find api -name "*.pb.go" -delete
	@find api -name "*_grpc.pb.go" -delete
	@find api -name "*_http.pb.go" -delete
	@find internal -name "*.pb.go" -delete
	@echo "Proto files cleaned successfully"

.PHONY: wire
# generate wire code
wire:
	wire gen ./cmd/QuotaLane/...

.PHONY: test
# run unit tests and integration tests
test:
	go test -v -race -cover -coverprofile=coverage.out ./...

.PHONY: lint
# run golangci-lint
lint:
	golangci-lint run --timeout=5m

.PHONY: docker
# build docker image
docker:
	docker build -t quotalane:$(VERSION) .

.PHONY: migrate
# run database migrations
migrate:
	@echo "Running database migrations..."
	@bash scripts/migrate.sh up

.PHONY: migrate-up
# migrate database up one version
migrate-up:
	@bash scripts/migrate.sh up 1

.PHONY: migrate-down
# migrate database down one version
migrate-down:
	@bash scripts/migrate.sh down 1

.PHONY: seed
# seed database with initial data
seed:
	@echo "Seeding database with initial data..."
	@bash scripts/seed.sh

.PHONY: redis-cli
# open Redis CLI in container
redis-cli:
	@docker exec -it quotalane-redis redis-cli

.PHONY: redis-flush
# flush all Redis cache data
redis-flush:
	@echo "Flushing all Redis cache data..."
	@docker exec -it quotalane-redis redis-cli FLUSHALL
	@echo "Redis cache cleared successfully"

.PHONY: all
# generate all
all:
	make api;
	make config;
	make generate;

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
