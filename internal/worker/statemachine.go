package worker

import "fmt"

const (
	WorkerDormant    WorkerStatus = "dormant"
	WorkerSpawning   WorkerStatus = "spawning"
	WorkerTrustCheck WorkerStatus = "trust_check"
	WorkerReady      WorkerStatus = "ready"
	WorkerBlocked    WorkerStatus = "blocked"
	WorkerReviewing  WorkerStatus = "reviewing"
	WorkerRecovery   WorkerStatus = "recovery"
	WorkerFailed     WorkerStatus = "failed"
)

var validTransitions = map[WorkerStatus][]WorkerStatus{
	WorkerDormant:    {WorkerSpawning, WorkerReady},
	WorkerSpawning:   {WorkerTrustCheck, WorkerReady, WorkerFailed},
	WorkerTrustCheck: {WorkerReady, WorkerFailed},
	WorkerReady:      {WorkerWorking, WorkerDormant, WorkerPaused},
	WorkerWorking:    {WorkerReviewing, WorkerBlocked, WorkerFailed, WorkerFinished, WorkerError},
	WorkerBlocked:    {WorkerReady, WorkerWorking, WorkerFailed},
	WorkerReviewing:  {WorkerReady, WorkerWorking, WorkerFailed},
	WorkerFailed:     {WorkerRecovery, WorkerDormant},
	WorkerRecovery:   {WorkerReady, WorkerFailed},
	WorkerPaused:     {WorkerReady, WorkerDormant},
	WorkerFinished:   {WorkerReady, WorkerDormant},
	WorkerError:      {WorkerRecovery, WorkerDormant, WorkerReady},
	WorkerIdle:       {WorkerWorking, WorkerDormant, WorkerSpawning, WorkerPaused},
	WorkerWaiting:    {WorkerReady, WorkerWorking, WorkerFailed},
}

type StateMachine struct {
	current WorkerStatus
}

func NewStateMachine(initial WorkerStatus) *StateMachine {
	return &StateMachine{current: initial}
}

func (sm *StateMachine) Current() WorkerStatus {
	return sm.current
}

func (sm *StateMachine) Transition(to WorkerStatus) error {
	allowed, ok := validTransitions[sm.current]
	if !ok {
		return fmt.Errorf("unknown state: %s", sm.current)
	}
	for _, a := range allowed {
		if a == to {
			sm.current = to
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s → %s", sm.current, to)
}

func (sm *StateMachine) CanTransition(to WorkerStatus) bool {
	allowed, ok := validTransitions[sm.current]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}
