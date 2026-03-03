import { writable } from 'svelte/store'

export const projects = writable([])

export async function loadProjects() {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const result = await window.go.gui.CompanyApp.ListProjects()
    projects.set(result || [])
  }
}

export async function createProject(name, description, repoPath, baseBranch, goals) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const p = await window.go.gui.CompanyApp.CreateProject(name, description, repoPath, baseBranch, goals)
    await loadProjects()
    return p
  }
}

export async function deleteProject(projectID) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    await window.go.gui.CompanyApp.DeleteProject(projectID)
    await loadProjects()
  }
}

export async function chatCreateProject(messages) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    return await window.go.gui.CompanyApp.ChatCreateProject(messages)
  }
}
