package worker

import (
	"strings"
	"sync"
	"time"
)

type RecoveryScenario string

const (
	RecoveryNone           RecoveryScenario = ""
	RecoveryTrustPrompt    RecoveryScenario = "trust_prompt"
	RecoveryPromptMissed   RecoveryScenario = "prompt_missed"
	RecoveryStaleBranch    RecoveryScenario = "stale_branch"
	RecoveryCompileError   RecoveryScenario = "compile_error"
	RecoverySessionDead    RecoveryScenario = "session_dead"
	RecoveryTokenExhausted RecoveryScenario = "token_exhausted"
)

type EscalationPolicy string

const (
	EscalateAlertHuman     EscalationPolicy = "alert_human"
	EscalateLogAndContinue EscalationPolicy = "log_and_continue"
	EscalateAbort          EscalationPolicy = "abort"
)

var scenarioEscalation = map[RecoveryScenario]EscalationPolicy{
	RecoveryTrustPrompt:    EscalateAlertHuman,
	RecoveryPromptMissed:   EscalateLogAndContinue,
	RecoveryStaleBranch:    EscalateAlertHuman,
	RecoveryCompileError:   EscalateAlertHuman,
	RecoverySessionDead:    EscalateAbort,
	RecoveryTokenExhausted: EscalateAlertHuman,
}

type recoveryRecord struct {
	Attempts int
	LastAt   time.Time
}

type RecoveryManager struct {
	MaxAttempts int
	mu          sync.Mutex
	records     map[string]map[RecoveryScenario]*recoveryRecord
}

func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		MaxAttempts: 1,
		records:     make(map[string]map[RecoveryScenario]*recoveryRecord),
	}
}

func (rm *RecoveryManager) DetectScenario(paneContent string) RecoveryScenario {
	lower := strings.ToLower(paneContent)
	switch {
	case strings.Contains(lower, "trust this project") || strings.Contains(lower, "do you trust"):
		return RecoveryTrustPrompt
	case strings.Contains(lower, "not a git repository") || strings.Contains(lower, "merge conflict"):
		return RecoveryStaleBranch
	case strings.Contains(lower, "compilation failed") || strings.Contains(lower, "build failed"):
		return RecoveryCompileError
	case strings.Contains(lower, "tokens remaining: 0") || strings.Contains(lower, "rate limit"):
		return RecoveryTokenExhausted
	default:
		return RecoveryNone
	}
}

func (rm *RecoveryManager) ShouldRetry(workerID string, scenario RecoveryScenario) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	workerRecords, ok := rm.records[workerID]
	if !ok {
		return true
	}
	rec, ok := workerRecords[scenario]
	if !ok {
		return true
	}
	return rec.Attempts < rm.MaxAttempts
}

func (rm *RecoveryManager) RecordAttempt(workerID string, scenario RecoveryScenario) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.records[workerID] == nil {
		rm.records[workerID] = make(map[RecoveryScenario]*recoveryRecord)
	}
	rec, ok := rm.records[workerID][scenario]
	if !ok {
		rec = &recoveryRecord{}
		rm.records[workerID][scenario] = rec
	}
	rec.Attempts++
	rec.LastAt = time.Now()
}

func (rm *RecoveryManager) GetEscalation(scenario RecoveryScenario) EscalationPolicy {
	if p, ok := scenarioEscalation[scenario]; ok {
		return p
	}
	return EscalateAlertHuman
}

func (rm *RecoveryManager) ClearRecords(workerID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.records, workerID)
}
