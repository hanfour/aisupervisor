package growth

import "strings"

type BossFeedback struct {
	WorkerID string
	TaskID   string
	Type     FeedbackType
	Content  string
	Branches []SkillBranch
}

type FeedbackType string

const (
	FeedbackPraise     FeedbackType = "praise"
	FeedbackCorrection FeedbackType = "correction"
	FeedbackPreference FeedbackType = "preference"
)

// ClassifyFeedback uses simple heuristics to classify boss messages.
func ClassifyFeedback(message string) FeedbackType {
	lower := strings.ToLower(message)
	praiseWords := []string{"good", "great", "excellent", "nice", "perfect", "well done",
		"好", "很好", "太好", "不錯", "讚", "優秀", "完美", "做得好"}
	correctionWords := []string{"wrong", "incorrect", "fix", "don't", "stop", "shouldn't",
		"錯", "不對", "不要", "改", "修", "別"}

	for _, w := range praiseWords {
		if strings.Contains(lower, w) {
			return FeedbackPraise
		}
	}
	for _, w := range correctionWords {
		if strings.Contains(lower, w) {
			return FeedbackCorrection
		}
	}
	return FeedbackPreference
}

// InferBranches guesses which skill branches a feedback message relates to.
func InferBranches(message string) []SkillBranch {
	lower := strings.ToLower(message)
	var branches []SkillBranch

	branchKeywords := map[SkillBranch][]string{
		BranchFrontend:     {"css", "html", "ui", "component", "svelte", "react", "前端", "介面", "樣式"},
		BranchBackend:      {"api", "database", "go", "server", "backend", "後端", "資料庫"},
		BranchSecurity:     {"security", "owasp", "xss", "injection", "安全", "漏洞"},
		BranchDevOps:       {"deploy", "ci", "docker", "k8s", "pipeline", "部署", "容器"},
		BranchArchitecture: {"architecture", "design", "pattern", "solid", "架構", "設計"},
		BranchResearch:     {"research", "analysis", "report", "研究", "分析", "報告"},
	}

	for branch, keywords := range branchKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				branches = append(branches, branch)
				break
			}
		}
	}

	return branches
}
