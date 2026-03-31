DATABASE_URL ?= postgres://postgres:postgres@localhost:5433/trainingops_test?sslmode=disable

.PHONY: test unit integration test-integration up-test-db down-test-db

unit:
	go test ./... -v

up-test-db:
	docker-compose -f docker-compose.test.yml up -d

down-test-db:
	docker-compose -f docker-compose.test.yml down

integration: up-test-db
	DATABASE_URL=$(DATABASE_URL) go test -tags=integration ./... -v

test-integration: integration

test: unit integration
