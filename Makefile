.PHONY: generate unit integration verify compatibility-safe compatibility-paid compatibility-mutation

generate:
	go generate ./...

unit:
	go test ./...

integration:
	npm --prefix tests/sdk/openai-node ci
	go test -tags=integration ./tests/integration/openai
	go test -race -tags=integration ./tests/integration/openai

verify: generate
	git diff --exit-code
	go test ./...
	go test -race ./...
	go vet ./...
	modernize ./...
	npm --prefix tests/sdk/openai-node ci
	go test -tags=integration ./tests/integration/openai

compatibility-safe:
	@test "$(OPENAI_COMPAT_PROFILE)" = safe
	go test -tags=compatibility -v ./tests/compatibility/openai

compatibility-paid:
	@test "$(OPENAI_COMPAT_PROFILE)" = paid
	@test "$(OPENAI_COMPAT_ALLOW_COST)" = 1
	go test -tags=compatibility -v ./tests/compatibility/openai

compatibility-mutation:
	@test "$(OPENAI_COMPAT_PROFILE)" = mutation
	@test "$(OPENAI_COMPAT_ALLOW_COST)" = 1
	@test "$(OPENAI_COMPAT_ALLOW_MUTATION)" = 1
	go test -tags=compatibility -v ./tests/compatibility/openai
