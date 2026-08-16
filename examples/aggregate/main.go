package main

import (
	"log"
	"net/http"

	aiservershell "github.com/idy/ai-server-shell"
	"github.com/idy/ai-server-shell/backend"
	openaihandler "github.com/idy/ai-server-shell/openai"
)

func main() {
	// Protocol handlers are constructed independently. A future Anthropic or
	// Gemini handler can be mounted beside OpenAI while sharing its Services.
	services, err := backend.NewServices()
	if err != nil {
		log.Fatal(err)
	}
	openAIHandler, err := openaihandler.NewHandler(services)
	if err != nil {
		log.Fatal(err)
	}
	shell, err := aiservershell.New(aiservershell.WithHandler("/v1/", openAIHandler))
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", shell))
}
