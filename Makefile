.PHONY: generate unit integration verify compatibility-safe compatibility-full

generate:
	go generate ./...

unit:
	go test ./...

integration:
	npm --prefix tests/sdk/openai-node ci
	go test -tags=integration ./tests/integration/openai

verify: generate
	git diff --exit-code
	go test ./...
	go test -race ./...
	go vet ./...
	npm --prefix tests/sdk/openai-node ci
	go test -tags=integration ./tests/integration/openai

compatibility-safe:
	@test "$(OPENAI_COMPAT_PROFILE)" = safe
	go test -tags=compatibility -v ./tests/compatibility/openai

compatibility-full:
	@test "$(OPENAI_COMPAT_PROFILE)" = full
	@test "$(OPENAI_COMPAT_ALLOW_MUTATION)" = 1
	go test -tags=compatibility -v ./tests/compatibility/openai
