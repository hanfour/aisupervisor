package policy

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Engine struct {
	mu       sync.RWMutex
	policies []Policy
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) AddPolicy(p Policy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies = append(e.policies, p)
}

func (e *Engine) SetPolicies(policies []Policy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies = policies
}

func (e *Engine) Evaluate(ctx EvalContext) []Action {
	e.mu.RLock()
	defer e.mu.RUnlock()

	type prioritizedAction struct {
		priority int
		action   Action
	}

	var results []prioritizedAction
	for _, p := range e.policies {
		if !p.Enabled {
			continue
		}
		if evaluateCondition(p.Condition, ctx) {
			results = append(results, prioritizedAction{p.Priority, p.Action})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].priority < results[j].priority
	})

	actions := make([]Action, len(results))
	for i, r := range results {
		actions[i] = r.action
	}
	return actions
}

func evaluateCondition(c Condition, ctx EvalContext) bool {
	switch c.Type {
	case ConditionAnd:
		for _, child := range c.Children {
			if !evaluateCondition(child, ctx) {
				return false
			}
		}
		return true
	case ConditionOr:
		for _, child := range c.Children {
			if evaluateCondition(child, ctx) {
				return true
			}
		}
		return false
	default:
		return evaluateSingle(c, ctx)
	}
}

func evaluateSingle(c Condition, ctx EvalContext) bool {
	val, ok := ctx[c.Field]
	if !ok {
		return false
	}
	return compare(val, c.Operator, c.Value)
}

func compare(actual interface{}, operator string, expected interface{}) bool {
	actualNum, actualIsNum := toFloat64(actual)
	expectedNum, expectedIsNum := toFloat64(expected)

	switch operator {
	case "eq":
		if actualIsNum && expectedIsNum {
			return actualNum == expectedNum
		}
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "gt":
		if actualIsNum && expectedIsNum {
			return actualNum > expectedNum
		}
		return false
	case "lt":
		if actualIsNum && expectedIsNum {
			return actualNum < expectedNum
		}
		return false
	case "gte":
		if actualIsNum && expectedIsNum {
			return actualNum >= expectedNum
		}
		return false
	case "lte":
		if actualIsNum && expectedIsNum {
			return actualNum <= expectedNum
		}
		return false
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected))
	default:
		return false
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}
