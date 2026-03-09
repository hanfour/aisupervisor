import { writable } from 'svelte/store'

export const tasks = writable([])

export async function loadTasks(projectID) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const result = await window.go.gui.CompanyApp.ListTasks(projectID)
    tasks.set(result || [])
  }
}

export async function createTask(projectID, title, description, prompt, dependsOn, priority, milestone, taskType = 'code') {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    const t = await window.go.gui.CompanyApp.CreateTask(projectID, title, description, prompt, dependsOn, priority, milestone, taskType)
    await loadTasks(projectID)
    return t
  }
}

export async function assignTask(workerID, taskID, projectID) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    await window.go.gui.CompanyApp.AssignTask(workerID, taskID)
    await loadTasks(projectID)
  }
}

export async function completeTask(taskID, projectID) {
  if (window.go && window.go.gui && window.go.gui.CompanyApp) {
    await window.go.gui.CompanyApp.CompleteTask(taskID)
    await loadTasks(projectID)
  }
}

export async function updateTaskStatus(taskID, status, projectID) {
  if (!window.go?.gui?.CompanyApp) return
  await window.go.gui.CompanyApp.UpdateTaskStatus(taskID, status)
  if (projectID) await loadTasks(projectID)
}

export async function reassignTask(taskID, newWorkerID, projectID) {
  if (!window.go?.gui?.CompanyApp) return
  await window.go.gui.CompanyApp.ReassignTask(taskID, newWorkerID)
  if (projectID) await loadTasks(projectID)
}
