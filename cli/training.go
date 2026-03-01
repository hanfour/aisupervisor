package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/training"
	"github.com/spf13/cobra"
)

var trainingCmd = &cobra.Command{
	Use:   "training",
	Short: "Manage training data and fine-tuning",
}

// --- Export ---

var trainingExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export training data for fine-tuning",
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		outputDir, _ := cmd.Flags().GetString("output")
		valRatio, _ := cmd.Flags().GetFloat64("val-ratio")
		verdicts, _ := cmd.Flags().GetString("verdicts")
		modelVer, _ := cmd.Flags().GetString("model-version")

		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		filter := training.ExportFilter{
			ModelVersion: modelVer,
		}
		if verdicts != "" {
			for _, v := range splitComma(verdicts) {
				filter.Verdicts = append(filter.Verdicts, training.Verdict(v))
			}
		}

		exporter := training.NewExporter(dataDir)
		if err := exporter.Export(training.ExportFormat(format), filter, valRatio, outputDir); err != nil {
			return err
		}
		fmt.Printf("Exported %s dataset to %s\n", format, outputDir)
		return nil
	},
}

// --- Models ---

var trainingModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List model versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		registry, err := training.NewModelRegistry(dataDir)
		if err != nil {
			return err
		}

		versions := registry.List()
		if len(versions) == 0 {
			fmt.Println("No model versions registered.")
			return nil
		}

		fmt.Printf("%-8s %-20s %-10s %-8s %-6s %-20s %s\n",
			"VERSION", "BASE", "METHOD", "PAIRS", "BENCH", "OLLAMA", "CREATED")
		for _, v := range versions {
			fmt.Printf("%-8s %-20s %-10s %-8d %-6.2f %-20s %s\n",
				v.Version, v.BaseModel, v.Method, v.TrainPairs,
				v.BenchmarkScore, v.OllamaModel, v.CreatedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var trainingModelChainCmd = &cobra.Command{
	Use:   "model-chain [version]",
	Short: "Show version lineage for a model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		registry, err := training.NewModelRegistry(dataDir)
		if err != nil {
			return err
		}

		chain := registry.VersionChain(args[0])
		if len(chain) == 0 {
			return fmt.Errorf("version %q not found", args[0])
		}

		for i, v := range chain {
			prefix := strings.Repeat("  ", i)
			fmt.Printf("%s%s (base: %s, method: %s, pairs: %d)\n",
				prefix, v.Version, v.BaseModel, v.Method, v.TrainPairs)
		}
		return nil
	},
}

// --- Finetune ---

var trainingFinetuneCmd = &cobra.Command{
	Use:   "finetune",
	Short: "Launch a fine-tune job",
	RunE: func(cmd *cobra.Command, args []string) error {
		method, _ := cmd.Flags().GetString("method")
		baseModel, _ := cmd.Flags().GetString("base-model")
		outputModel, _ := cmd.Flags().GetString("output-model")
		scriptPath, _ := cmd.Flags().GetString("script")
		valRatio, _ := cmd.Flags().GetFloat64("val-ratio")

		if baseModel == "" {
			return fmt.Errorf("--base-model is required")
		}

		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		registry, err := training.NewModelRegistry(dataDir)
		if err != nil {
			return err
		}
		exporter := training.NewExporter(dataDir)
		runner := training.NewFinetuneRunner(dataDir, registry, exporter)

		cfg := training.FinetuneConfig{
			Method:      method,
			BaseModel:   baseModel,
			OutputModel: outputModel,
			ScriptPath:  scriptPath,
			ValRatio:    valRatio,
		}

		job, err := runner.Launch(cfg)
		if err != nil {
			return err
		}
		fmt.Printf("Fine-tune job launched: %s (status: %s)\n", job.ID, job.Status)
		return nil
	},
}

// --- Benchmark ---

var trainingBenchmarkGenCmd = &cobra.Command{
	Use:   "benchmark-gen",
	Short: "Generate a benchmark suite from approved reviews",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		maxTasks, _ := cmd.Flags().GetInt("max-tasks")

		if name == "" {
			name = "default"
		}

		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		gen := training.NewBenchmarkGenerator(dataDir)
		suite, err := gen.Generate(name, maxTasks)
		if err != nil {
			return err
		}
		fmt.Printf("Benchmark suite generated: %s (%d tasks)\n", suite.ID, len(suite.Tasks))
		return nil
	},
}

var trainingBenchmarkListCmd = &cobra.Command{
	Use:   "benchmark-list",
	Short: "List benchmark suites",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		gen := training.NewBenchmarkGenerator(dataDir)
		suites, err := gen.ListSuites()
		if err != nil {
			return err
		}
		if len(suites) == 0 {
			fmt.Println("No benchmark suites.")
			return nil
		}
		for _, id := range suites {
			fmt.Println(id)
		}
		return nil
	},
}

// --- Stats ---

var trainingStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show training data statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := trainingDataDir()
		if err != nil {
			return err
		}

		// Count review pairs
		path := filepath.Join(dataDir, "review_pairs.jsonl")
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No training data yet.")
				return nil
			}
			return err
		}

		// Quick line count
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := 0
		for _, b := range data {
			if b == '\n' {
				lines++
			}
		}

		fmt.Printf("Review pairs: %d\n", lines)
		fmt.Printf("Data file: %s (%.1f KB)\n", path, float64(info.Size())/1024)

		// Model versions
		registry, err := training.NewModelRegistry(dataDir)
		if err == nil {
			fmt.Printf("Model versions: %d\n", len(registry.List()))
		}

		// Benchmarks
		gen := training.NewBenchmarkGenerator(dataDir)
		if suites, err := gen.ListSuites(); err == nil {
			fmt.Printf("Benchmark suites: %d\n", len(suites))
		}

		return nil
	},
}

func trainingDataDir() (string, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return "", err
	}
	if cfg.Training.DataDir != "" {
		dir := cfg.Training.DataDir
		// Expand ~
		if strings.HasPrefix(dir, "~/") {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[2:])
		}
		return dir, nil
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "aisupervisor", "training"), nil
}

func init() {
	// Export
	trainingExportCmd.Flags().String("format", "sft", "Export format (sft|dpo)")
	trainingExportCmd.Flags().String("output", "./training-export", "Output directory")
	trainingExportCmd.Flags().Float64("val-ratio", 0.1, "Validation split ratio")
	trainingExportCmd.Flags().String("verdicts", "", "Filter by verdicts (comma-separated: accepted,rejected,revised)")
	trainingExportCmd.Flags().String("model-version", "", "Filter by model version")

	// Finetune
	trainingFinetuneCmd.Flags().String("method", "sft", "Fine-tune method (sft|dpo)")
	trainingFinetuneCmd.Flags().String("base-model", "", "Base model name")
	trainingFinetuneCmd.Flags().String("output-model", "", "Output Ollama model name")
	trainingFinetuneCmd.Flags().String("script", "", "Custom training script path")
	trainingFinetuneCmd.Flags().Float64("val-ratio", 0.1, "Validation split ratio")

	// Benchmark gen
	trainingBenchmarkGenCmd.Flags().String("name", "default", "Benchmark suite name")
	trainingBenchmarkGenCmd.Flags().Int("max-tasks", 50, "Max benchmark tasks")

	// Build tree
	trainingCmd.AddCommand(trainingExportCmd)
	trainingCmd.AddCommand(trainingModelsCmd)
	trainingCmd.AddCommand(trainingModelChainCmd)
	trainingCmd.AddCommand(trainingFinetuneCmd)
	trainingCmd.AddCommand(trainingBenchmarkGenCmd)
	trainingCmd.AddCommand(trainingBenchmarkListCmd)
	trainingCmd.AddCommand(trainingStatsCmd)
	rootCmd.AddCommand(trainingCmd)
}
