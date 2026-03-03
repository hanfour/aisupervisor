package personality

import (
	"context"
	"testing"
)

type mockAIBackend struct {
	response string
}

func (m *mockAIBackend) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func TestNarrator_GeneratePersonality(t *testing.T) {
	mock := &mockAIBackend{
		response: `{"description":"安靜但專注的工程師","catchphrases":["先喝杯咖啡","這個 bug 有趣"],"backstory":"曾在遊戲公司工作"}`,
	}
	n := NewNarrator(mock)

	traits := PersonalityTraits{
		Sociability: 30, Focus: 85, Creativity: 60,
		Empathy: 50, Ambition: 70, Humor: 40,
	}
	narrative, err := n.GeneratePersonality(context.Background(), "小明", traits)
	if err != nil {
		t.Fatalf("GeneratePersonality: %v", err)
	}
	if narrative.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(narrative.Catchphrases) == 0 {
		t.Error("Catchphrases should not be empty")
	}
}

func TestNarrator_GenerateDialogue(t *testing.T) {
	mock := &mockAIBackend{response: "先喝杯咖啡再來看這個 bug"}
	n := NewNarrator(mock)

	p := NewCharacterProfile("w1")
	p.Narrative.Catchphrases = []string{"先喝杯咖啡"}

	dialogue, err := n.GenerateDialogue(context.Background(), p, "watercooler")
	if err != nil {
		t.Fatalf("GenerateDialogue: %v", err)
	}
	if dialogue == "" {
		t.Error("Dialogue should not be empty")
	}
}
