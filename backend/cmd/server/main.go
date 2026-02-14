package main

import (
	"log"
	"net/http"
	"os"

	"github.com/behavox/argocd-book-plugin/internal/handler"
	"github.com/behavox/argocd-book-plugin/internal/k8s"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("failed to create k8s client: %v", err)
	}

	h := handler.New(client)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	log.Printf("starting booking backend on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
