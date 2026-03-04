package company

import "github.com/hanfourmini/aisupervisor/internal/worker"

// WorkerOption configures optional fields when creating a worker.
type WorkerOption func(*worker.Worker)

func WithTier(tier worker.WorkerTier) WorkerOption {
	return func(w *worker.Worker) {
		w.Tier = tier
	}
}

func WithParent(parentID string) WorkerOption {
	return func(w *worker.Worker) {
		w.ParentID = parentID
	}
}

func WithBackend(backendID string) WorkerOption {
	return func(w *worker.Worker) {
		w.BackendID = backendID
	}
}

func WithCLITool(tool string) WorkerOption {
	return func(w *worker.Worker) {
		w.CLITool = tool
	}
}

func WithModelVersion(v string) WorkerOption {
	return func(w *worker.Worker) {
		w.ModelVersion = v
	}
}

func WithSkillProfile(profile string) WorkerOption {
	return func(w *worker.Worker) {
		w.SkillProfile = profile
	}
}

func WithGender(gender worker.WorkerGender) WorkerOption {
	return func(w *worker.Worker) {
		w.Gender = gender
	}
}
