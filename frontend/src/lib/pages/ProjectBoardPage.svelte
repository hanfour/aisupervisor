<script>
  import { onMount, onDestroy } from 'svelte'
  import { tasks, loadTasks, assignTask, completeTask, updateTaskStatus, reassignTask } from '../stores/tasks.js'
  import { workers, loadWorkers } from '../stores/workers.js'
  import TaskCard from '../components/TaskCard.svelte'
  import TaskForm from '../components/TaskForm.svelte'
  import ResearchReportCard from '../components/ResearchReportCard.svelte'
  import { addError } from '../stores/errors.js'
  import { t } from '../stores/i18n.js'

  export let projectId = ''
  export let onNavigate = () => {}

  let project = null
  let progress = null
  let showTaskForm = false
  let assignDialog = null
  let selectedWorker = ''
  let dragTaskId = null
  let dragOverCol = null
  let viewingReport = null
  let reportWorkerName = ''
  let eventCleanup = null

  const columnDefs = [
    { key: 'backlog', i18nKey: 'board.backlog', statuses: ['backlog'], dropStatus: 'backlog' },
    { key: 'ready', i18nKey: 'board.ready', statuses: ['ready'], dropStatus: 'ready' },
    { key: 'progress', i18nKey: 'board.inProgress', statuses: ['assigned', 'in_progress'], dropStatus: 'in_progress' },
    { key: 'review', i18nKey: 'board.review', statuses: ['review'], dropStatus: 'review' },
    { key: 'escalation', i18nKey: 'board.escalation', statuses: ['escalation'], dropStatus: 'escalation' },
    { key: 'done', i18nKey: 'board.done', statuses: ['done', 'failed'], dropStatus: 'done' },
  ]

  $: tasksByColumn = columnDefs.map(col => ({
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

    if (window.runtime) {
      eventCleanup = window.runtime.EventsOn('company:event', async () => {
        if (projectId) {
          await loadTasks(projectId)
          progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
        }
        await loadWorkers()
      })
    }
  })

  onDestroy(() => {
    if (eventCleanup) eventCleanup()
  })

  function handleAssign(task) {
    assignDialog = task
    selectedWorker = ''
  }

  async function confirmAssign() {
    if (!selectedWorker || !assignDialog) return
    try {
      if (assignDialog._reassign) {
        await reassignTask(assignDialog.id, selectedWorker, projectId)
      } else {
        await assignTask(selectedWorker, assignDialog.id, projectId)
      }
      await loadWorkers()
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (e) {
      addError('Failed to assign task: ' + (e.message || e))
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

  async function handleViewReport(task) {
    try {
      const report = await window.go.gui.CompanyApp.GetReport(task.id)
      if (report) {
        const assignee = ($workers || []).find(w => w.id === task.assigneeId)
        reportWorkerName = assignee ? assignee.name : ''
        viewingReport = report
      } else {
        addError('No report found for this task')
      }
    } catch (e) {
      addError('Failed to load report: ' + e.message)
    }
  }

  function handleReassign(task) {
    assignDialog = task
    assignDialog._reassign = true
    selectedWorker = ''
  }

  async function confirmReassign() {
    if (!selectedWorker || !assignDialog) return
    try {
      await reassignTask(assignDialog.id, selectedWorker, projectId)
      await loadWorkers()
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (e) {
      addError('Failed to reassign task: ' + (e.message || e))
    }
    assignDialog = null
  }

  async function handleEscalate(task) {
    if (!confirm($t('task.escalateConfirm'))) return
    try {
      await updateTaskStatus(task.id, 'escalation', projectId)
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (e) {
      addError('Failed to escalate task: ' + (e.message || e))
    }
  }

  async function handleMarkFailed(task) {
    if (!confirm($t('task.markFailedConfirm'))) return
    try {
      await updateTaskStatus(task.id, 'failed', projectId)
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (e) {
      addError('Failed to mark task as failed: ' + (e.message || e))
    }
  }

  // --- Drag and Drop ---
  function handleDragStart(e, taskId) {
    dragTaskId = taskId
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', taskId)
  }

  function handleDragEnd() {
    dragTaskId = null
    dragOverCol = null
  }

  function handleDragOver(e, colKey) {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    dragOverCol = colKey
  }

  function handleDragLeave(colKey) {
    if (dragOverCol === colKey) dragOverCol = null
  }

  async function handleDrop(e, col) {
    e.preventDefault()
    dragOverCol = null
    const taskId = e.dataTransfer.getData('text/plain')
    if (!taskId) return

    try {
      await updateTaskStatus(taskId, col.dropStatus, projectId)
      progress = await window.go.gui.CompanyApp.GetProjectProgress(projectId)
    } catch (err) {
      addError('Failed to move task: ' + err.message)
    }
    dragTaskId = null
  }
</script>

<div class="board-page">
  <div class="board-header nes-container is-dark">
    <div class="header-left">
      <button class="nes-btn btn-sm" on:click={() => onNavigate('projects')}>&larr;</button>
      <span class="proj-title">{project?.name || 'Project Board'}</span>
    </div>
    {#if progress}
      <div class="progress-bar">
        <progress class="nes-progress is-primary" value={progress.percent} max="100"></progress>
        <span class="progress-label">{progress.done}/{progress.total} done</span>
      </div>
    {/if}
    <button class="nes-btn is-primary btn-sm" on:click={() => showTaskForm = true}>{$t('board.addTask')}</button>
  </div>

  <div class="kanban">
    {#each tasksByColumn as col}
      <div
        class="kanban-col"
        class:drag-over={dragOverCol === col.key}
        on:dragover={(e) => handleDragOver(e, col.key)}
        on:dragleave={() => handleDragLeave(col.key)}
        on:drop={(e) => handleDrop(e, col)}
        role="list"
      >
        <div class="col-header">
          <span class="col-title">{$t(col.i18nKey)}</span>
          <span class="col-count">{col.tasks.length}</span>
        </div>
        <div class="col-body">
          {#each col.tasks as task (task.id)}
            <div
              class="task-drag-wrap"
              class:dragging={dragTaskId === task.id}
              draggable="true"
              on:dragstart={(e) => handleDragStart(e, task.id)}
              on:dragend={handleDragEnd}
              role="listitem"
            >
              <TaskCard
                {task}
                workers={$workers}
                onAssign={handleAssign}
                onComplete={handleComplete}
                onViewReport={handleViewReport}
                onReassign={handleReassign}
                onEscalate={handleEscalate}
                onMarkFailed={handleMarkFailed}
              />
            </div>
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

  <ResearchReportCard
    report={viewingReport}
    workerName={reportWorkerName}
    onClose={() => viewingReport = null}
  />

  {#if assignDialog}
    <div class="dialog-overlay" on:click={() => assignDialog = null} on:keydown={(e) => e.key === 'Escape' && (assignDialog = null)} role="presentation">
      <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
        <p class="title">{assignDialog._reassign ? 'Reassign' : 'Assign'}: {assignDialog.title}</p>
        {#if (assignDialog._reassign ? $workers : idleWorkers).length === 0}
          <p class="empty-msg">{$t('workers.noWorkers')}</p>
        {:else}
          <div class="worker-list">
            {#each (assignDialog._reassign ? $workers : idleWorkers) as w}
              <label class="worker-option" class:selected={selectedWorker === w.id}>
                <input type="radio" class="nes-radio is-dark" name="worker" value={w.id} bind:group={selectedWorker} />
                <span>{w.name}</span>
              </label>
            {/each}
          </div>
        {/if}
        <div class="dialog-actions">
          <button class="nes-btn is-primary" disabled={!selectedWorker} on:click={confirmAssign}>{$t('common.assign')}</button>
          <button class="nes-btn" on:click={() => assignDialog = null}>{$t('common.cancel')}</button>
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
    transition: border-color 0.15s;
  }

  .kanban-col.drag-over {
    border: 2px dashed var(--accent-blue);
    background: rgba(0, 212, 255, 0.05);
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

  .task-drag-wrap {
    cursor: grab;
    transition: opacity 0.15s;
  }

  .task-drag-wrap:active {
    cursor: grabbing;
  }

  .task-drag-wrap.dragging {
    opacity: 0.4;
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
