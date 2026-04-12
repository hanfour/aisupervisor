package policy

type ConditionType string

const (
	ConditionSingle ConditionType = ""
	ConditionAnd    ConditionType = "and"
	ConditionOr     ConditionType = "or"
)

type ActionType string

const (
	ActionBlock       ActionType = "block_assignment"
	ActionEmitEvent   ActionType = "emit_event"
	ActionCooldown    ActionType = "set_cooldown"
	ActionSuggestPair ActionType = "suggest_pair"
	ActionNotifyBoss  ActionType = "notify_boss"
)

type Condition struct {
	Type     ConditionType `yaml:"type,omitempty" json:"type,omitempty"`
	Field    string        `yaml:"field,omitempty" json:"field,omitempty"`
	Operator string        `yaml:"operator,omitempty" json:"operator,omitempty"`
	Value    interface{}   `yaml:"value,omitempty" json:"value,omitempty"`
	Children []Condition   `yaml:"children,omitempty" json:"children,omitempty"`
}

type Action struct {
	Type   ActionType             `yaml:"type" json:"type"`
	Params map[string]interface{} `yaml:"params,omitempty" json:"params,omitempty"`
}

type Policy struct {
	ID        string    `yaml:"id" json:"id"`
	Name      string    `yaml:"name" json:"name"`
	Priority  int       `yaml:"priority" json:"priority"`
	Condition Condition `yaml:"condition" json:"condition"`
	Action    Action    `yaml:"action" json:"action"`
	Enabled   bool      `yaml:"enabled" json:"enabled"`
}

type EvalContext map[string]interface{}
