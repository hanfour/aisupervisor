package training

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// FinetuneConfig holds parameters for a fine-tune job.
type FinetuneConfig struct {
	Method       string `yaml:"method" json:"method"`               // "sft" or "dpo"
	BaseModel    string `yaml:"base_model" json:"base_model"`       // e.g. "codellama:7b"
	OutputModel  string `yaml:"output_model" json:"output_model"`   // ollama model name
	ScriptPath   string `yaml:"script_path" json:"script_path"`     // custom script (optional)
	AutoTrigger  int    `yaml:"auto_trigger" json:"auto_trigger"`   // min pairs before auto-trigger (0=disabled)
	ValRatio     float64 `yaml:"val_ratio" json:"val_ratio"`        // validation split ratio
}

// FinetuneJob represents a running or completed fine-tune job.
type FinetuneJob struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"` // "pending", "running", "completed", "failed"
	Config       FinetuneConfig `json:"config"`
	DatasetPath  string    `json:"dataset_path"`
	OutputVersion string   `json:"output_version"`
	PairsUsed    int       `json:"pairs_used"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// FinetuneRunner manages fine-tune jobs.
type FinetuneRunner struct {
	mu       sync.Mutex
	dataDir  string
	registry *ModelRegistry
	exporter *Exporter
	jobs     []FinetuneJob
	counter  int64
}

func NewFinetuneRunner(dataDir string, registry *ModelRegistry, exporter *Exporter) *FinetuneRunner {
	return &FinetuneRunner{
		dataDir:  dataDir,
		registry: registry,
		exporter: exporter,
	}
}

// CheckAutoTrigger checks if enough pairs have accumulated to trigger auto fine-tune.
func (r *FinetuneRunner) CheckAutoTrigger(cfg FinetuneConfig) (bool, error) {
	if cfg.AutoTrigger <= 0 {
		return false, nil
	}

	count, err := r.countPairs()
	if err != nil {
		return false, err
	}

	// Check how many pairs were used in the last fine-tune
	lastUsed := 0
	if latest := r.registry.Latest(); latest != nil {
		lastUsed = latest.TrainPairs
	}

	return count-lastUsed >= cfg.AutoTrigger, nil
}

// Launch starts a fine-tune job.
func (r *FinetuneRunner) Launch(cfg FinetuneConfig) (*FinetuneJob, error) {
	r.mu.Lock()
	r.counter++
	jobID := fmt.Sprintf("ft-%d-%d", time.Now().UnixMilli(), r.counter)

	job := FinetuneJob{
		ID:        jobID,
		Status:    "pending",
		Config:    cfg,
		StartedAt: time.Now(),
	}
	r.jobs = append(r.jobs, job)
	r.mu.Unlock()

	// Export dataset
	valRatio := cfg.ValRatio
	if valRatio <= 0 {
		valRatio = 0.1
	}
	outputDir := filepath.Join(r.dataDir, "datasets", jobID)
	format := FormatSFT
	if cfg.Method == "dpo" {
		format = FormatDPO
	}

	if err := r.exporter.Export(format, ExportFilter{}, valRatio, outputDir); err != nil {
		r.updateJob(jobID, "failed", err.Error())
		return r.getJob(jobID), fmt.Errorf("exporting dataset: %w", err)
	}

	r.updateJobDataset(jobID, outputDir)

	// Count pairs used
	pairsUsed, _ := r.countPairs()

	// Run fine-tune in background
	go r.runFinetune(jobID, cfg, outputDir, pairsUsed)

	return r.getJob(jobID), nil
}

func (r *FinetuneRunner) runFinetune(jobID string, cfg FinetuneConfig, datasetPath string, pairsUsed int) {
	r.updateJob(jobID, "running", "")

	var err error
	if cfg.ScriptPath != "" {
		err = r.runCustomScript(cfg.ScriptPath, datasetPath, cfg)
	} else {
		err = r.runLlamaFactory(datasetPath, cfg)
	}

	if err != nil {
		r.updateJob(jobID, "failed", err.Error())
		return
	}

	// Register new model with ollama
	if cfg.OutputModel != "" {
		if err := r.ollamaCreate(cfg.OutputModel, cfg.BaseModel); err != nil {
			r.updateJob(jobID, "failed", fmt.Sprintf("ollama create: %v", err))
			return
		}
	}

	// Determine version
	parentVer := ""
	if latest := r.registry.Latest(); latest != nil {
		parentVer = latest.Version
	}
	version := fmt.Sprintf("v%d", len(r.registry.List())+1)

	// Register in model registry
	r.registry.Register(ModelVersion{
		Version:     version,
		BaseModel:   cfg.BaseModel,
		ParentVer:   parentVer,
		TrainPairs:  pairsUsed,
		Method:      cfg.Method,
		OllamaModel: cfg.OutputModel,
		CreatedAt:   time.Now(),
	})

	r.updateJobComplete(jobID, version)
}

func (r *FinetuneRunner) runLlamaFactory(datasetPath string, cfg FinetuneConfig) error {
	args := []string{"train"}
	// llamafactory-cli expects a config file or arguments
	cmd := exec.Command("llamafactory-cli", args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DATASET_DIR=%s", datasetPath),
		fmt.Sprintf("MODEL_NAME=%s", cfg.BaseModel),
		fmt.Sprintf("OUTPUT_MODEL=%s", cfg.OutputModel),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *FinetuneRunner) runCustomScript(scriptPath, datasetPath string, cfg FinetuneConfig) error {
	cmd := exec.Command(scriptPath, datasetPath, cfg.BaseModel, cfg.OutputModel)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *FinetuneRunner) ollamaCreate(modelName, baseModel string) error {
	// Create a Modelfile for the fine-tuned model
	modelfileContent := fmt.Sprintf("FROM %s\n", baseModel)
	modelfilePath := filepath.Join(r.dataDir, "Modelfile.tmp")
	if err := os.WriteFile(modelfilePath, []byte(modelfileContent), 0o644); err != nil {
		return err
	}
	defer os.Remove(modelfilePath)

	cmd := exec.Command("ollama", "create", modelName, "-f", modelfilePath)
	return cmd.Run()
}

func (r *FinetuneRunner) countPairs() (int, error) {
	path := filepath.Join(r.dataDir, "review_pairs.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var pair ReviewPair
		if json.Unmarshal(scanner.Bytes(), &pair) == nil {
			count++
		}
	}
	return count, scanner.Err()
}

func (r *FinetuneRunner) updateJob(id, status, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].Status = status
			r.jobs[i].Error = errMsg
			return
		}
	}
}

func (r *FinetuneRunner) updateJobDataset(id, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].DatasetPath = path
			return
		}
	}
}

func (r *FinetuneRunner) updateJobComplete(id, version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].Status = "completed"
			r.jobs[i].OutputVersion = version
			r.jobs[i].CompletedAt = &now
			return
		}
	}
}

func (r *FinetuneRunner) getJob(id string) *FinetuneJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			j := r.jobs[i]
			return &j
		}
	}
	return nil
}

// ListJobs returns all fine-tune jobs.
func (r *FinetuneRunner) ListJobs() []FinetuneJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]FinetuneJob, len(r.jobs))
	copy(result, r.jobs)
	return result
}
