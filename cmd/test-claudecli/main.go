package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/ai/claudecli"
)

func main() {
	os.Unsetenv("CLAUDECODE")
	cli := claudecli.New()
	if cli == nil {
		fmt.Println("FAIL: Claude CLI not found")
		return
	}
	fmt.Println("OK: Found claude at:", cli.Path())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msgs := []ai.ChatMessage{
		{Role: "system", Content: "Reply with only: {\"status\":\"chatting\",\"message\":\"hello\"}"},
		{Role: "user", Content: "hi"},
	}
	fmt.Println("Calling claude -p ...")
	start := time.Now()
	resp, err := cli.Chat(ctx, msgs)
	elapsed := time.Since(start)
	fmt.Printf("Elapsed: %v\n", elapsed)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	if len(resp) > 300 {
		resp = resp[:300]
	}
	fmt.Println("RESPONSE:", resp)
}
