package growth

// MigrateWorkerSkillTree initializes a SkillTree for a worker if missing.
// It maps existing personality skill scores to initial EXP.
func MigrateWorkerSkillTree(currentTree *SkillTree, skillScores map[string]int) *SkillTree {
	if currentTree != nil {
		return currentTree // already migrated
	}
	st := NewSkillTree()

	// Map existing personality SkillScores to initial EXP
	if skillScores != nil {
		if v, ok := skillScores["codeQuality"]; ok {
			st.Branches[BranchBackend].AddEXP(v / 2)
		}
		if v, ok := skillScores["securityAwareness"]; ok {
			st.Branches[BranchSecurity].AddEXP(v / 2)
		}
		if v, ok := skillScores["carefulness"]; ok {
			st.Branches[BranchFrontend].AddEXP(v / 3)
		}
		if v, ok := skillScores["efficiency"]; ok {
			st.Branches[BranchDevOps].AddEXP(v / 3)
		}
		if v, ok := skillScores["learnability"]; ok {
			st.Branches[BranchResearch].AddEXP(v / 3)
		}
		if v, ok := skillScores["creativity"]; ok {
			st.Branches[BranchArchitecture].AddEXP(v / 3)
		}
	}

	return st
}
