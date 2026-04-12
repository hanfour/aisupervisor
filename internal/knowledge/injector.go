package knowledge

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Injector struct {
	store          *Store
	maxTokenBudget int
}

func NewInjector(store *Store, maxTokenBudget int) *Injector {
	if maxTokenBudget <= 0 {
		maxTokenBudget = 2000
	}
	return &Injector{store: store, maxTokenBudget: maxTokenBudget}
}

func tierCharBudget(tier KnowledgeTier) int {
	switch tier {
	case TierL0Identity:
		return 250
	case TierL1Essential:
		return 1250
	case TierL2RoomRecall:
		return 3750
	case TierL3DeepSearch:
		return 7500
	default:
		return 3750
	}
}

func (inj *Injector) BuildContext(workerID, projectID string, tier KnowledgeTier) (string, error) {
	all, err := inj.store.GetAll(workerID, projectID)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", nil
	}

	// Filter entries by tier: only include entries whose tier <= requested tier
	var filtered []Entry
	for _, e := range all {
		if TierForType(e.Type) <= tier {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return "", nil
	}

	type scored struct {
		entry Entry
		score float64
	}
	now := time.Now()
	var items []scored
	for _, e := range filtered {
		recency := recencyScore(e.CreatedAt, now)
		access := math.Log2(float64(e.AccessCount + 1))
		score := e.Relevance*0.5 + recency*0.3 + access*0.2
		items = append(items, scored{e, score})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	charBudget := tierCharBudget(tier)
	if maxBudget := inj.maxTokenBudget * 5; maxBudget < charBudget {
		charBudget = maxBudget
	}
	var parts []string
	used := 0

	header := "## Project Knowledge\n"
	used += len(header)

	for _, item := range items {
		line := fmt.Sprintf("- [%s] %s", item.entry.Type, item.entry.Summary)
		if tier >= TierL3DeepSearch && item.entry.FullContent != "" {
			line += "\n  " + item.entry.FullContent
		}
		if used+len(line)+1 > charBudget {
			break
		}
		parts = append(parts, line)
		used += len(line) + 1
	}

	if len(parts) == 0 {
		return "", nil
	}

	return header + strings.Join(parts, "\n"), nil
}

func recencyScore(created time.Time, now time.Time) float64 {
	hours := now.Sub(created).Hours()
	return math.Exp(-hours / (7 * 24) * math.Ln2)
}
