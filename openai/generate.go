package openai

//go:generate go run ../internal/cmd/openai-gen -spec ../spec/openai/openapi.json -events ../spec/openai/realtime-events.json -output internal/profile/profile.gen.go -manifest ../tests/sdk/openai-node/operations.json -compatibility ../spec/openai/compatibility.yaml
