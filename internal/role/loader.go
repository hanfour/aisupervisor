package role

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hanfourmini/aisupervisor/internal/ai"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"gopkg.in/yaml.v3"
)

// LoadFromConfig builds roles from config role entries.
func LoadFromConfig(cfgs []config.RoleConfig, defaultBackend ai.Backend) []Role {
	var roles []Role
	for _, cfg := range cfgs {
		if !cfg.Enabled {
			continue
		}
		// The built-in gatekeeper ID is handled specially
		if cfg.ID == "permission_gatekeeper" {
			// Gatekeeper is created separately; skip it here to avoid duplicates
			continue
		}
		r, err := NewAIRole(cfg, defaultBackend)
		if err != nil {
			log.Printf("warning: failed to create role %q: %v", cfg.ID, err)
			continue
		}
		roles = append(roles, r)
	}
	return roles
}

// LoadFromDir scans a directory for YAML role definitions.
func LoadFromDir(dir string, defaultBackend ai.Backend) []Role {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var roles []Role
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			log.Printf("warning: failed to read role file %s: %v", entry.Name(), err)
			continue
		}

		var cfg config.RoleConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Printf("warning: failed to parse role file %s: %v", entry.Name(), err)
			continue
		}

		if !cfg.Enabled {
			continue
		}

		r, err := NewAIRole(cfg, defaultBackend)
		if err != nil {
			log.Printf("warning: failed to create role from %s: %v", entry.Name(), err)
			continue
		}
		roles = append(roles, r)
	}
	return roles
}
