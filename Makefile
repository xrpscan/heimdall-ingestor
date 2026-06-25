SHELL=/usr/bin/env bash

application_name        = ingestor
application_binary_name = ingestor

# Support both podman and docker.
DOCKER=$(shell which podman || which docker || echo 'docker')

# Builds the project.
build:
	@echo "+$@"
	@go build -o bin/$(application_binary_name) cmd/$(application_name)/main.go

# Runs the project after linting and building it anew.
run: tidy build
	@echo "+$@"
	@echo "########### Running the application binary ############"
	@bin/$(application_binary_name)

# Tests the whole project.
test:
	@echo "+$@"
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Runs the "go mod tidy" command.
tidy:
	@echo "+$@"
	@go mod tidy

# Runs golang-ci-lint over the project.
lint:
	@echo "+$@"
	@golangci-lint run ./...

# Builds the docker image for the project.
image:
	@echo "+$@"
	@$(DOCKER) build --file Containerfile --tag $(application_name):latest .

# Runs the project container assuming the image is already built.
container:
	@echo "+$@"
	@echo "############### Removing old container ################"
	@$(DOCKER) rm -f $(application_name)

	@echo "################ Running new container ################"
	@$(DOCKER) run --name $(application_name) --detach --publish 9983:9983 \
        --volume $(PWD)/config/config.docker.json:/service/config/config.json \
        --volume $(HOME)/.secure-kafka/tls/ca-cert:/service/tls/ca-cert:ro \
        --volume $(PWD)/logs:/service/logs:z \
        $(application_name):latest

migrate-up:
	@echo "+$@"
	@migrate -verbose -path db/migrations -database $(database_dsn) up

migrate-down:
	@echo "+$@"
	@migrate -verbose -path db/migrations -database $(database_dsn) down
