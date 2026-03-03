package personality

const (
	EventTaskCompleted  = "task_completed"
	EventTaskFailed     = "task_failed"
	EventReviewRejected = "review_rejected"
	EventReviewApproved = "review_approved"
	EventSocialActivity = "social_activity"
	EventWatercooler    = "watercooler"
	EventComforted      = "comforted"
	EventWorkTick       = "work_tick"
)

type moodRule struct {
	MoraleDelta int
	EnergyDelta int
	NewMood     string
}

var baseRules = map[string]moodRule{
	EventTaskCompleted:  {MoraleDelta: 10, EnergyDelta: -15, NewMood: MoodExcited},
	EventTaskFailed:     {MoraleDelta: -20, EnergyDelta: -10, NewMood: MoodFrustrated},
	EventReviewRejected: {MoraleDelta: -10, EnergyDelta: -5, NewMood: MoodStressed},
	EventReviewApproved: {MoraleDelta: 5, EnergyDelta: 0, NewMood: MoodHappy},
	EventSocialActivity: {MoraleDelta: 5, EnergyDelta: -3, NewMood: ""},
	EventWatercooler:    {MoraleDelta: 2, EnergyDelta: 10, NewMood: ""},
	EventComforted:      {MoraleDelta: 8, EnergyDelta: 5, NewMood: ""},
	EventWorkTick:       {MoraleDelta: 0, EnergyDelta: -2, NewMood: ""},
}

func ApplyEvent(p *CharacterProfile, event string) {
	rule, ok := baseRules[event]
	if !ok {
		return
	}

	moraleDelta := rule.MoraleDelta

	if event == EventSocialActivity && p.Traits.Sociability > 70 {
		moraleDelta = 8
	}

	p.Mood.AdjustMorale(moraleDelta)
	p.Mood.AdjustEnergy(rule.EnergyDelta)

	if rule.NewMood != "" {
		p.Mood.Current = rule.NewMood
	}
}

func UpdateAutoMood(p *CharacterProfile) {
	if p.Mood.Energy < 20 {
		p.Mood.Current = MoodTired
	}
}
