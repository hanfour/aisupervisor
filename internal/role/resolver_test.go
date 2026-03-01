package role

import (
	"context"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/config"
)

// resolverTestRole implements Role for resolver tests.
type resolverTestRole struct {
	id       string
	name     string
	mode     Mode
	priority int
}

func (r *resolverTestRole) ID() string                                                    { return r.id }
func (r *resolverTestRole) Name() string                                                  { return r.name }
func (r *resolverTestRole) Mode() Mode                                                    { return r.mode }
func (r *resolverTestRole) Priority() int                                                 { return r.priority }
func (r *resolverTestRole) ShouldEvaluate(Observation) bool                               { return true }
func (r *resolverTestRole) Evaluate(context.Context, Observation) (*Intervention, error) { return nil, nil }

func TestResolver_WithBinding(t *testing.T) {
	r1 := &resolverTestRole{id: "gatekeeper", name: "Gatekeeper", mode: ModeReactive, priority: 100}
	r2 := &resolverTestRole{id: "rdm", name: "RD Manager", mode: ModeReactive, priority: 80}
	r3 := &resolverTestRole{id: "security", name: "Security", mode: ModeHybrid, priority: 90}

	mgr := NewManager(r1, r2, r3)
	bindings := []config.SessionRoleBinding{
		{SessionID: "sess1", RoleIDs: []string{"gatekeeper", "security"}},
	}
	resolver := NewResolver(mgr, bindings)

	roles := resolver.RolesForSession("sess1")
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles for sess1, got %d", len(roles))
	}

	ids := map[string]bool{}
	for _, r := range roles {
		ids[r.ID()] = true
	}
	if !ids["gatekeeper"] || !ids["security"] {
		t.Errorf("expected gatekeeper and security, got %v", ids)
	}
	if ids["rdm"] {
		t.Error("rdm should not be in sess1's roles")
	}
}

func TestResolver_NoBinding_ReturnsAll(t *testing.T) {
	r1 := &resolverTestRole{id: "gatekeeper", name: "Gatekeeper", mode: ModeReactive, priority: 100}
	r2 := &resolverTestRole{id: "rdm", name: "RD Manager", mode: ModeReactive, priority: 80}

	mgr := NewManager(r1, r2)
	resolver := NewResolver(mgr, nil)

	roles := resolver.RolesForSession("unknown-session")
	if len(roles) != 2 {
		t.Fatalf("expected all 2 roles for unbound session, got %d", len(roles))
	}
}

func TestResolver_SetSessionRoles(t *testing.T) {
	r1 := &resolverTestRole{id: "gatekeeper", name: "Gatekeeper", mode: ModeReactive, priority: 100}
	r2 := &resolverTestRole{id: "rdm", name: "RD Manager", mode: ModeReactive, priority: 80}

	mgr := NewManager(r1, r2)
	resolver := NewResolver(mgr, nil)

	// Initially no binding → all roles
	roles := resolver.RolesForSession("sess1")
	if len(roles) != 2 {
		t.Fatalf("expected 2, got %d", len(roles))
	}

	// Set binding
	resolver.SetSessionRoles("sess1", []string{"rdm"})
	roles = resolver.RolesForSession("sess1")
	if len(roles) != 1 {
		t.Fatalf("expected 1 after binding, got %d", len(roles))
	}
	if roles[0].ID() != "rdm" {
		t.Errorf("expected rdm, got %s", roles[0].ID())
	}

	// Clear binding
	resolver.SetSessionRoles("sess1", nil)
	roles = resolver.RolesForSession("sess1")
	if len(roles) != 2 {
		t.Fatalf("expected 2 after clear, got %d", len(roles))
	}
}

func TestResolver_GetSessionRoleIDs(t *testing.T) {
	mgr := NewManager()
	bindings := []config.SessionRoleBinding{
		{SessionID: "sess1", RoleIDs: []string{"a", "b"}},
	}
	resolver := NewResolver(mgr, bindings)

	ids := resolver.GetSessionRoleIDs("sess1")
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Errorf("expected [a b], got %v", ids)
	}

	ids2 := resolver.GetSessionRoleIDs("unknown")
	if ids2 != nil {
		t.Errorf("expected nil for unknown session, got %v", ids2)
	}
}
