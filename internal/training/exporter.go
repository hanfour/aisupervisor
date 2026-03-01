package training

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

// ExportFormat defines the output format for training data.
type ExportFormat string

const (
	FormatSFT ExportFormat = "sft"
	FormatDPO ExportFormat = "dpo"
)

// ExportFilter controls which review pairs are included.
type ExportFilter struct {
	Verdicts     []Verdict // empty = all
	ModelVersion string    // empty = all
	MinDurationMs int64
}

// SFTEntry is the Llama Factory SFT JSON format.
type SFTEntry struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input"`
	Output      string `json:"output"`
}

// DPOEntry is the Llama Factory DPO JSON format.
type DPOEntry struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input"`
	Chosen      string `json:"chosen"`
	Rejected    string `json:"rejected"`
}

// Exporter reads JSONL review pairs and exports training datasets.
type Exporter struct {
	dataDir string
}

func NewExporter(dataDir string) *Exporter {
	return &Exporter{dataDir: dataDir}
}

// Export reads review pairs, applies filters, and writes train/val splits.
func (e *Exporter) Export(format ExportFormat, filter ExportFilter, valRatio float64, outputDir string) error {
	pairs, err := e.readPairs(filter)
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return fmt.Errorf("no review pairs match filter")
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	// Shuffle for random split
	rand.Shuffle(len(pairs), func(i, j int) {
		pairs[i], pairs[j] = pairs[j], pairs[i]
	})

	splitIdx := int(float64(len(pairs)) * (1 - valRatio))
	if splitIdx < 1 {
		splitIdx = 1
	}
	trainPairs := pairs[:splitIdx]
	valPairs := pairs[splitIdx:]

	switch format {
	case FormatSFT:
		if err := e.writeSFT(trainPairs, filepath.Join(outputDir, "train_sft.json")); err != nil {
			return err
		}
		if len(valPairs) > 0 {
			return e.writeSFT(valPairs, filepath.Join(outputDir, "val_sft.json"))
		}
	case FormatDPO:
		if err := e.writeDPO(trainPairs, filepath.Join(outputDir, "train_dpo.json")); err != nil {
			return err
		}
		if len(valPairs) > 0 {
			return e.writeDPO(valPairs, filepath.Join(outputDir, "val_dpo.json"))
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}

func (e *Exporter) readPairs(filter ExportFilter) ([]ReviewPair, error) {
	path := filepath.Join(e.dataDir, "review_pairs.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening JSONL: %w", err)
	}
	defer f.Close()

	verdictSet := make(map[Verdict]bool)
	for _, v := range filter.Verdicts {
		verdictSet[v] = true
	}

	var pairs []ReviewPair
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		var pair ReviewPair
		if err := json.Unmarshal(scanner.Bytes(), &pair); err != nil {
			continue // skip malformed lines
		}
		if len(verdictSet) > 0 && !verdictSet[pair.Verdict] {
			continue
		}
		if filter.ModelVersion != "" && pair.ModelVersion != filter.ModelVersion {
			continue
		}
		if filter.MinDurationMs > 0 && pair.DurationMs < filter.MinDurationMs {
			continue
		}
		pairs = append(pairs, pair)
	}
	return pairs, scanner.Err()
}

func (e *Exporter) writeSFT(pairs []ReviewPair, path string) error {
	entries := make([]SFTEntry, 0, len(pairs))
	for _, p := range pairs {
		entries = append(entries, SFTEntry{
			Instruction: p.Prompt,
			Input:       p.DiffSummary,
			Output:      p.ManagerOutput,
		})
	}
	return writeJSON(path, entries)
}

func (e *Exporter) writeDPO(pairs []ReviewPair, path string) error {
	entries := make([]DPOEntry, 0, len(pairs))
	for _, p := range pairs {
		entries = append(entries, DPOEntry{
			Instruction: p.Prompt,
			Input:       p.DiffSummary,
			Chosen:      p.ManagerOutput,
			Rejected:    p.EngineerOutput,
		})
	}
	return writeJSON(path, entries)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
