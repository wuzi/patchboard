.PHONY: sqlc test run web build e2e

sqlc:
	go tool sqlc generate

test:
	go test ./...
	cd web && npx ng test --watch=false

run:
	PB_BASE_URL=http://localhost:4200 PB_DATA_DIR=./data go run ./cmd/patchboard serve

web/node_modules: web/package-lock.json
	cd web && npm ci --no-audit --no-fund
	touch web/node_modules

web: web/node_modules
	cd web && npm run build

build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/patchboard ./cmd/patchboard

e2e/node_modules: e2e/package-lock.json
	cd e2e && npm ci --no-audit --no-fund && npx playwright install chromium
	touch e2e/node_modules

e2e: e2e/node_modules
	cd e2e && npx playwright test
