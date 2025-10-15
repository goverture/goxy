package main

import (
	"log"
	"net/http"
	"os"

	"fake-openai/handlers"
)

func main() {
	startFakeServer()
}

func startFakeServer() {
	http.HandleFunc("/v1/chat/completions", handlers.HandleChatCompletions)
	http.HandleFunc("/v1/responses", handlers.HandleResponses)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	port = ":" + port

	log.Printf("Starting fake OpenAI server on http://localhost%s", port)
	log.Printf("Use this as your OpenAI base URL: http://localhost%s", port)
	log.Printf("Available endpoints:")
	log.Printf("  POST %s/v1/chat/completions", port)
	log.Printf("  POST %s/v1/responses", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
