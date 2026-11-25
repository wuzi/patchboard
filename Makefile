.PHONY: sqlc test run

sqlc:
	go tool sqlc generate

test:
	go test ./...

run:
	PB_BASE_URL=http://localhost:4200 PB_DATA_DIR=./data go run ./cmd/patchboard serve
