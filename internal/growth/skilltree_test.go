package growth

import "testing"

func TestNewSkillTree(t *testing.T) {
	st := NewSkillTree()
	if len(st.Branches) != 6 {
		t.Errorf("expected 6 branches, got %d", len(st.Branches))
	}
	for _, b := range AllBranches {
		node, ok := st.Branches[b]
		if !ok {
			t.Errorf("missing branch %s", b)
		}
		if node.Level != 1 {
			t.Errorf("branch %s should start at level 1, got %d", b, node.Level)
		}
	}
}

func TestSkillNode_AddEXP_LevelUp(t *testing.T) {
	st := NewSkillTree()
	node := st.Branches[BranchFrontend]
	leveled := node.AddEXP(120)
	if node.Level != 2 {
		t.Errorf("expected level 2 after 120 EXP, got %d", node.Level)
	}
	if node.CurrentEXP != 20 {
		t.Errorf("expected 20 overflow EXP, got %d", node.CurrentEXP)
	}
	if !leveled {
		t.Error("should return true on level up")
	}
}

func TestSkillNode_AddEXP_MaxLevel(t *testing.T) {
	st := NewSkillTree()
	node := st.Branches[BranchBackend]
	node.Level = 5
	node.TotalEXP = 2750
	leveled := node.AddEXP(9999)
	if node.Level != 5 {
		t.Error("should not exceed level 5")
	}
	if leveled {
		t.Error("should not level up at max")
	}
}

func TestSkillTree_DominantBranch(t *testing.T) {
	st := NewSkillTree()
	st.Branches[BranchSecurity].Level = 4
	st.Branches[BranchSecurity].TotalEXP = 900
	dominant := st.DominantBranch()
	if dominant != BranchSecurity {
		t.Errorf("expected security as dominant, got %s", dominant)
	}
}
