package growth

import "testing"

func TestMigrateWorkerSkillTree_AlreadyMigrated(t *testing.T) {
	existing := NewSkillTree()
	existing.Branches[BranchBackend].Level = 3
	result := MigrateWorkerSkillTree(existing, nil)
	if result != existing {
		t.Error("should return existing tree when already migrated")
	}
}

func TestMigrateWorkerSkillTree_NilTree(t *testing.T) {
	scores := map[string]int{
		"codeQuality":       200,
		"securityAwareness": 100,
		"carefulness":       90,
	}
	st := MigrateWorkerSkillTree(nil, scores)
	if st == nil {
		t.Fatal("should create new skill tree")
	}
	if st.Branches[BranchBackend].TotalEXP != 100 {
		t.Errorf("backend should have 100 EXP from codeQuality/2, got %d", st.Branches[BranchBackend].TotalEXP)
	}
	if st.Branches[BranchSecurity].TotalEXP != 50 {
		t.Errorf("security should have 50 EXP from securityAwareness/2, got %d", st.Branches[BranchSecurity].TotalEXP)
	}
	if st.Branches[BranchFrontend].TotalEXP != 30 {
		t.Errorf("frontend should have 30 EXP from carefulness/3, got %d", st.Branches[BranchFrontend].TotalEXP)
	}
}

func TestMigrateWorkerSkillTree_NilScores(t *testing.T) {
	st := MigrateWorkerSkillTree(nil, nil)
	if st == nil {
		t.Fatal("should create new skill tree even with nil scores")
	}
	for _, b := range AllBranches {
		if st.Branches[b].Level != 1 {
			t.Errorf("branch %s should be level 1, got %d", b, st.Branches[b].Level)
		}
	}
}

func TestMigrateWorkerSkillTree_LevelUp(t *testing.T) {
	scores := map[string]int{
		"codeQuality": 400, // 400/2 = 200 EXP -> should level up from 1 to 2 (threshold 100)
	}
	st := MigrateWorkerSkillTree(nil, scores)
	if st.Branches[BranchBackend].Level != 2 {
		t.Errorf("backend should be level 2 after 200 EXP, got %d", st.Branches[BranchBackend].Level)
	}
}
