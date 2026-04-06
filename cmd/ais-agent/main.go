package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/agent/providers"
	"github.com/hanfourmini/aisupervisor/internal/agent/tools"
)

var version = "0.1.0"

func main() {
	providerName := flag.String("provider", "openai", "LLM provider (openai, anthropic, ollama)")
	model := flag.String("model", "", "Model name")
	apiKey := flag.String("api-key", "", "API key (default: from env)")
	baseURL := flag.String("base-url", "", "Override API base URL")
	systemPrompt := flag.String("append-system-prompt", "", "Additional system prompt")
	allowedTools := flag.String("allowed-tools", "", "Comma-separated allowed tools")
	maxTokens := flag.Int("max-tokens", 0, "Max context tokens")
	maxIterations := flag.Int("max-iterations", 200, "Max loop iterations")
	permissionMode := flag.String("permission-mode", "bypassPermissions", "plan, acceptEdits, bypassPermissions")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ais-agent v%s\n", version)
		os.Exit(0)
	}

	if *apiKey == "" {
		switch *providerName {
		case "openai":
			*apiKey = os.Getenv("OPENAI_API_KEY")
		case "anthropic":
			*apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	}

	provider, err := createProvider(*providerName, *apiKey, *model, *baseURL)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	registry := tools.NewRegistry()
	registerTools(registry, *permissionMode)

	if *allowedTools != "" {
		registry.SetAllowed(strings.Split(*allowedTools, ","))
	}

	sysPrompt := "You are an expert software engineer. Use the available tools to complete tasks. When done, provide a summary of what you did."
	if *systemPrompt != "" {
		sysPrompt += "\n\n" + *systemPrompt
	}

	tokenBudget := *maxTokens
	if tokenBudget == 0 {
		tokenBudget = provider.MaxContextTokens()
	}

	loop := agent.NewLoop(provider, registry, sysPrompt, tokenBudget)
	loop.MaxIterations = *maxIterations
	loop.OnOutput = func(text string) { fmt.Println(text) }
	loop.OnToolCall = func(name, args string) { fmt.Printf("  [tool] %s\n", name) }

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	modelName := *model
	if modelName == "" {
		modelName = "default"
	}
	fmt.Printf("ais-agent v%s (%s/%s)\n", version, provider.Name(), modelName)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for {
		fmt.Print("❯ ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/stop" || input == "/exit" {
			fmt.Println("Goodbye.")
			break
		}
		if input == "/reset" {
			loop.ResetHistory()
			fmt.Println("Conversation reset.")
			continue
		}

		if err := loop.Run(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Printf("  [tokens] in:%d out:%d\n", loop.TotalInput, loop.TotalOutput)
	}
}

// createProvider initializes an LLM provider by name.
func createProvider(name, apiKey, model, baseURL string) (agent.Provider, error) {
	switch name {
	case "openai":
		return providers.NewOpenAI(apiKey, model, baseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: openai)", name)
	}
}

// registerTools adds tools to the registry based on the permission mode.
func registerTools(registry *tools.Registry, permMode string) {
	// Read-only tools are always available.
	registry.Register(&tools.ReadTool{})
	registry.Register(&tools.GlobTool{})
	registry.Register(&tools.GrepTool{})

	// Write/execute tools depend on permission mode.
	switch permMode {
	case "acceptEdits", "bypassPermissions":
		registry.Register(&tools.EditTool{})
		registry.Register(&tools.WriteTool{})
		registry.Register(&tools.BashTool{})
	}
}
