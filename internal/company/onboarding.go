package company

import (
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// OnboardingConfig holds the wizard selections for first-time setup.
type OnboardingConfig struct {
	CompanyName  string `json:"companyName"`
	Language     string `json:"language"`     // "en" or "zh-TW"
	TeamTemplate string `json:"teamTemplate"` // "starter" | "full" | "custom"
	APIKeySource string `json:"apiKeySource"` // "oauth" | "api_key"
}

// teamMember describes a worker to create during onboarding.
type teamMember struct {
	Name         string
	Avatar       string
	Tier         worker.WorkerTier
	SkillProfile string
	Gender       worker.WorkerGender
}

// starterTeam: 1 manager (opus) + 2 engineers (sonnet)
var starterTeam = []teamMember{
	{Name: "Alice", Avatar: "👩‍💼", Tier: worker.TierManager, SkillProfile: "architect", Gender: worker.GenderFemale},
	{Name: "Bob", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderMale},
	{Name: "Carol", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderFemale},
}

// fullTeam: 1 consultant + 3 managers + 12 engineers
var fullTeam = []teamMember{
	// Consultant
	{Name: "Steve", Avatar: "🧑‍💼", Tier: worker.TierConsultant, SkillProfile: "analyst", Gender: worker.GenderMale},
	// Managers
	{Name: "Alice", Avatar: "👩‍💼", Tier: worker.TierManager, SkillProfile: "architect", Gender: worker.GenderFemale},
	{Name: "David", Avatar: "👨‍💼", Tier: worker.TierManager, SkillProfile: "architect", Gender: worker.GenderMale},
	{Name: "Eve", Avatar: "👩‍💼", Tier: worker.TierManager, SkillProfile: "architect", Gender: worker.GenderFemale},
	// Engineers
	{Name: "Bob", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderMale},
	{Name: "Carol", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderFemale},
	{Name: "Frank", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderMale},
	{Name: "Grace", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderFemale},
	{Name: "Hank", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderMale},
	{Name: "Ivy", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderFemale},
	{Name: "Jack", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "hacker", Gender: worker.GenderMale},
	{Name: "Kate", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderFemale},
	{Name: "Leo", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "devops", Gender: worker.GenderMale},
	{Name: "Mia", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "designer", Gender: worker.GenderFemale},
	{Name: "Nick", Avatar: "👨‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderMale},
	{Name: "Olivia", Avatar: "👩‍💻", Tier: worker.TierEngineer, SkillProfile: "coder", Gender: worker.GenderFemale},
}

// ApplyOnboarding sets up the team based on the wizard configuration.
func (m *Manager) ApplyOnboarding(cfg OnboardingConfig) error {
	// 1. Set language
	if cfg.Language != "" {
		m.SetLanguage(cfg.Language)
	}

	// 2. Select team template
	var team []teamMember
	switch cfg.TeamTemplate {
	case "starter":
		team = starterTeam
	case "full":
		team = fullTeam
	case "custom":
		// Custom mode: don't create workers, user will add manually
		return nil
	default:
		return fmt.Errorf("unknown team template %q", cfg.TeamTemplate)
	}

	// 3. Create workers with hierarchy: managers report to consultant, engineers report to first manager
	var consultantID, firstManagerID string

	for _, tm := range team {
		opts := []WorkerOption{
			WithTier(tm.Tier),
			WithSkillProfile(tm.SkillProfile),
			WithGender(tm.Gender),
		}

		// Set parent based on tier
		switch tm.Tier {
		case worker.TierManager:
			if consultantID != "" {
				opts = append(opts, WithParent(consultantID))
			}
		case worker.TierEngineer:
			if firstManagerID != "" {
				opts = append(opts, WithParent(firstManagerID))
			}
		}

		w, err := m.CreateWorker(tm.Name, tm.Avatar, opts...)
		if err != nil {
			return fmt.Errorf("creating worker %s: %w", tm.Name, err)
		}

		// Track IDs for hierarchy wiring
		switch tm.Tier {
		case worker.TierConsultant:
			consultantID = w.ID
		case worker.TierManager:
			if firstManagerID == "" {
				firstManagerID = w.ID
			}
		}
	}

	return nil
}
