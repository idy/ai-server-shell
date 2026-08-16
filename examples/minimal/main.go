package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/idy/ai-server-shell/backend"
	openaihandler "github.com/idy/ai-server-shell/openai"
)

type app struct{}

func (*app) Handle(_ context.Context, request backend.Request) (backend.Response, error) {
	if request.Operation == "listModels" {
		return backend.Response{JSON: json.RawMessage(`{"object":"list","data":[]}`)}, nil
	}
	return backend.Response{}, &backend.Error{Kind: backend.ErrorUnsupported, Message: "operation is not implemented"}
}

func main() {
	services, err := backend.NewServices(backend.WithModels(&app{}))
	if err != nil {
		log.Fatal(err)
	}
	handler, err := openaihandler.NewHandler(services)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", handler))
}
