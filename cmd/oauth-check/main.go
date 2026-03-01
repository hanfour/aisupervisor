package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai/anthropic"
)

func main() {
	fmt.Println("Testing OAuth backend connection...")
	be, err := anthropic.NewOAuthBackend("claude-oauth", "claude-sonnet-4-6")
	if err != nil {
		log.Fatalf("Failed to create OAuth backend: %v", err)
	}
	fmt.Println("OAuth backend created successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Println("Sending health check (ping)...")
	if err := be.Healthy(ctx); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Println("Health check PASSED - OAuth backend is working!")
}
