.PHONY: doctor check-server-structure check-toolchain server-build test test-server test-web test-launcher web-typecheck launcher-typecheck

doctor: check-toolchain check-server-structure

check-toolchain:
	python scripts/check-toolchain.py

check-server-structure:
	python scripts/check-server-structure.py

server-build: doctor
	cd server && mkdir -p dist && go build -o "dist/raylea-server$$(go env GOEXE)" ./cmd/raylea-server

test: test-server test-web test-launcher

test-server: doctor
	cd server && go test ./...

test-web: doctor
	cd web && corepack pnpm test

test-launcher: doctor
	cd launcher && corepack pnpm test

web-typecheck: doctor
	cd web && corepack pnpm run typecheck

launcher-typecheck: doctor
	cd launcher && corepack pnpm run typecheck
