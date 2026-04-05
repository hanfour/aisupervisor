// Growth store — skill tree, growth history, feedback, feature flags
import { writable } from 'svelte/store';

export const skillTree = writable(null);
export const growthHistory = writable([]);
export const featureFlags = writable([]);

export async function loadSkillTree(workerID) {
  try {
    const data = await window.go.gui.CompanyApp.GetSkillTree(workerID);
    skillTree.set(data);
    return data;
  } catch (e) {
    console.error('Failed to load skill tree:', e);
    return null;
  }
}

export async function loadGrowthHistory(workerID, limit = 50) {
  try {
    const data = await window.go.gui.CompanyApp.GetGrowthHistory(workerID, limit);
    growthHistory.set(data);
    return data;
  } catch (e) {
    console.error('Failed to load growth history:', e);
    return [];
  }
}

export async function submitFeedback(workerID, taskID, content) {
  return await window.go.gui.CompanyApp.SubmitBossFeedback(workerID, taskID, content);
}

export async function loadFeatureFlags() {
  try {
    const data = await window.go.gui.CompanyApp.GetFeatureFlags();
    featureFlags.set(data || []);
    return data;
  } catch (e) {
    console.error('Failed to load feature flags:', e);
    return [];
  }
}

export async function toggleFlag(id, enabled) {
  await window.go.gui.CompanyApp.ToggleFeatureFlag(id, enabled);
  await loadFeatureFlags();
}
