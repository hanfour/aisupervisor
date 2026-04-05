package policy

import "testing"

func TestEngine_Evaluate_SimpleCondition(t *testing.T) {
	engine := NewEngine()
	engine.AddPolicy(Policy{
		ID:       "block-low-skill",
		Name:     "Block low skill assignment",
		Priority: 10,
		Enabled:  true,
		Condition: Condition{
			Field:    "worker.level",
			Operator: "lt",
			Value:    3,
		},
		Action: Action{Type: ActionBlock, Params: map[string]interface{}{"reason": "skill too low"}},
	})

	ctx := EvalContext{"worker.level": 2}
	result := engine.Evaluate(ctx)
	if len(result) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result))
	}
	if result[0].Type != ActionBlock {
		t.Errorf("expected block action, got %s", result[0].Type)
	}
}

func TestEngine_Evaluate_AndCondition(t *testing.T) {
	engine := NewEngine()
	engine.AddPolicy(Policy{
		ID:       "cooldown",
		Enabled:  true,
		Priority: 20,
		Condition: Condition{
			Type: ConditionAnd,
			Children: []Condition{
				{Field: "worker.consecutiveFailures", Operator: "gt", Value: 2},
				{Field: "worker.status", Operator: "eq", Value: "ready"},
			},
		},
		Action: Action{Type: ActionCooldown, Params: map[string]interface{}{"duration": "30m"}},
	})

	ctx := EvalContext{
		"worker.consecutiveFailures": 3,
		"worker.status":              "ready",
	}
	result := engine.Evaluate(ctx)
	if len(result) != 1 {
		t.Fatal("expected cooldown action")
	}
}

func TestEngine_Evaluate_DisabledPolicy(t *testing.T) {
	engine := NewEngine()
	engine.AddPolicy(Policy{
		ID:      "disabled",
		Enabled: false,
		Condition: Condition{Field: "x", Operator: "eq", Value: 1},
		Action:    Action{Type: ActionBlock},
	})

	result := engine.Evaluate(EvalContext{"x": 1})
	if len(result) != 0 {
		t.Error("disabled policy should not trigger")
	}
}

func TestEngine_PriorityOrder(t *testing.T) {
	engine := NewEngine()
	engine.AddPolicy(Policy{ID: "low", Priority: 50, Enabled: true,
		Condition: Condition{Field: "x", Operator: "eq", Value: 1},
		Action:    Action{Type: ActionEmitEvent, Params: map[string]interface{}{"event": "low"}}})
	engine.AddPolicy(Policy{ID: "high", Priority: 10, Enabled: true,
		Condition: Condition{Field: "x", Operator: "eq", Value: 1},
		Action:    Action{Type: ActionBlock, Params: map[string]interface{}{"event": "high"}}})

	result := engine.Evaluate(EvalContext{"x": 1})
	if len(result) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(result))
	}
	if result[0].Type != ActionBlock {
		t.Error("higher priority (lower number) should come first")
	}
}
