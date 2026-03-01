<script>
  import { onMount, onDestroy } from 'svelte'
  import { tasks, loadTasks, assignTask, completeTask } from '../stores/tasks.js'
  import { workers, loadWorkers } from '../stores/workers.js'
  import TaskCard from '../components/TaskCard.svelte'
  import TaskForm from '../components/TaskForm.svelte'
  import { addError } from '../stores/errors.js'

  export let projectId = ''
  export let onNavigate = () => {}

  let project = null
  let progress = null
  let showTaskForm = false
  let assignDialog = null // task being assigned
  let selectedWorker = ''

  const columns = [
    { key: 'backlog', label: 'Backlog', statuses: ['backlog'] },
    { key: 'ready', label: 'Ready', statuses: ['ready'] },
    { key: 'progress', label: 'In Progress', statuses: ['assigned', 'in_progress'] },
    { key: 'review', label: 'Review', statuses: ['review'] },
    { key: 'done', label: 'Done', statuses: ['done', 'failed'] },
  ]

  $: tasksByColumn = columns.map(col => ({
    ...col,
    tasks: ($tasks || []).filter(t => col.statuses.includes(t.status))
  }))

  $: idleWorkers = ($workers || []).filter(w => w.status === 'idle')

  onMount(async () => {
    try {
      if (projectId) {
        const p = await window.go.gui.CompanyApp.GetProject(projectId)
        project = p
        await loadTasks(projectId)
        progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
      }
      await loadWorkers()
    } catch (e) {
      addError('Failed to load board: ' + e.message)
    }

    // Refresh on company events
    if (window.runtime) {
      window.runtime.EventsOn('company:event', async () => {
        if (projectId) {
          await loadTasks(projectId)
          progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
        }
        await loadWorkers()
      })
    }
  })

  function handleAssign(task) {
    assignDialog = task
    selectedWorker = ''
  }

  async function confirmAssign() {
    if (!selectedWorker || !assignDialog) return
    try {
      await assignTask(selectedWorker, assignDialog.id, projectId)
      await loadWorkers()
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (e) {
      addError('Failed to assign task: ' + e.message)
    }
    assignDialog = null
  }

  async function handleComplete(task) {
    try {
      await completeTask(task.id, projectId)
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (e) {
      addError('Failed to complete task: ' + e.message)
    }
  }
</script>

<div class="board-page">
  <div class="board-header nes-container is-dark">
    <div class="header-left">
      <button class="nes-btn btn-sm" on:click={() => onNavigate('projects')}>Back</button>
      <span class="proj-title">{project?.name || 'Project Board'}</span>
    </div>
    {#if progress}
      <div class="progress-bar">
        <progress class="nes-progress is-primary" value={progress.percent} max="100"></progress>
        <span class="progress-label">{progress.done}/{progress.total} done</span>
      </div>
    {/if}
    <button class="nes-btn is-primary btn-sm" on:click={() => showTaskForm = true}>+ Task</button>
  </div>

  <div class="kanban">
    {#each tasksByColumn as col}
      <div class="kanban-col">
        <div class="col-header">
          <span class="col-title">{col.label}</span>
          <span class="col-count">{col.tasks.length}</span>
        </div>
        <div class="col-body">
          {#each col.tasks as task}
            <TaskCard
              {task}
              workers={$workers}
              onAssign={handleAssign}
              onComplete={handleComplete}
            />
          {/each}
        </div>
      </div>
    {/each}
  </div>

  <TaskForm
    visible={showTaskForm}
    {projectId}
    existingTasks={$tasks || []}
    onClose={() => showTaskForm = false}
  />

  {#if assignDialog}
    <div class="dialog-overlay" on:click={() => assignDialog = null} on:keydown={(e) => e.key === 'Escape' && (assignDialog = null)} role="presentation">
      <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
        <p class="title">Assign: {assignDialog.title}</p>
        {#if idleWorkers.length === 0}
          <p class="empty-msg">No idle workers available</p>
        {:else}
          <div class="worker-list">
            {#each idleWorkers as w}
              <label class="worker-option" class:selected={selectedWorker === w.id}>
                <input type="radio" class="nes-radio is-dark" name="worker" value={w.id} bind:group={selectedWorker} />
                <span>{w.name}</span>
              </label>
            {/each}
          </div>
        {/if}
        <div class="dialog-actions">
          <button class="nes-btn is-primary" disabled={!selectedWorker} on:click={confirmAssign}>Assign</button>
          <button class="nes-btn" on:click={() => assignDialog = null}>Cancel</button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .board-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .board-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 12px !important;
    margin-bottom: 8px !important;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .proj-title {
    font-size: 12px;
    color: var(--accent-green);
  }

  .progress-bar {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .progress-bar progress {
    flex: 1;
    height: 16px;
  }

  .progress-label {
    font-size: 9px;
    white-space: nowrap;
  }

  .btn-sm {
    font-size: 9px !important;
    padding: 4px 8px !important;
  }

  .kanban {
    display: flex;
    gap: 8px;
    flex: 1;
    overflow-x: auto;
    overflow-y: hidden;
    padding-bottom: 8px;
  }

  .kanban-col {
    flex: 1;
    min-width: 200px;
    display: flex;
    flex-direction: column;
    border: 2px solid var(--border-color);
    overflow: hidden;
  }

  .col-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 8px;
    border-bottom: 2px solid var(--border-color);
    background: var(--bg-secondary);
  }

  .col-title {
    font-size: 10px;
    color: var(--accent-blue);
  }

  .col-count {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .col-body {
    flex: 1;
    overflow-y: auto;
    padding: 6px;
  }

  .dialog-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0,0,0,0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .nes-dialog {
    width: 360px;
    padding: 24px !important;
  }

  .worker-list {
    margin: 12px 0;
  }

  .worker-option {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 10px;
    margin: 6px 0;
    cursor: pointer;
  }

  .worker-option.selected {
    color: var(--accent-green);
  }

  .dialog-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .dialog-actions button {
    font-size: 10px;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
  }
</style>
