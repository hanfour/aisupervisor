<script>
  import BranchStatus from './BranchStatus.svelte'

  export let task = {}
  export let workers = []
  export let onAssign = () => {}
  export let onComplete = () => {}
  export let onViewReport = () => {}
  export let onReassign = () => {}
  export let onEscalate = () => {}
  export let onMarkFailed = () => {}

  $: assignee = workers.find(w => w.id === task.assigneeId)

  function priorityLabel(p) {
    if (p <= 1) return 'P1'
    if (p <= 2) return 'P2'
    if (p <= 3) return 'P3'
    return 'P' + p
  }

  function priorityClass(p) {
    if (p <= 1) return 'is-error'
    if (p <= 2) return 'is-warning'
    return 'is-primary'
  }
</script>

<div class="nes-container is-rounded task-card">
  <div class="card-header">
    <span class="task-title">
      {#if task.type === 'research'}
        <span class="type-badge research">R</span>
      {/if}
      {#if task.type === 'admin'}
        <span class="type-badge admin">A</span>
      {/if}
      {#if task.type === 'hr'}
        <span class="type-badge hr">HR</span>
      {/if}
      {#if task.rejectionCount > 0}
        <span class="type-badge rejection">R:{task.rejectionCount}</span>
      {/if}
      {task.title}
    </span>
    {#if task.priority}
      <span class="nes-badge"><span class={priorityClass(task.priority)}>{priorityLabel(task.priority)}</span></span>
    {/if}
  </div>

  {#if task.description}
    <p class="task-desc">{task.description}</p>
  {/if}

  {#if task.branchName}
    <BranchStatus branchName={task.branchName} />
  {/if}

  {#if assignee}
    <div class="assignee">
      <span class="label">Assigned:</span> {assignee.name}
    </div>
  {/if}

  {#if task.milestone}
    <div class="milestone">
      <span class="label">Milestone:</span> {task.milestone}
    </div>
  {/if}

  {#if task.dependsOn && task.dependsOn.length > 0}
    <div class="deps">
      <span class="label">Depends on:</span> {task.dependsOn.join(', ')}
    </div>
  {/if}

  <div class="card-actions">
    {#if task.status === 'ready'}
      <button class="nes-btn is-primary btn-sm" on:click={() => onAssign(task)}>Assign</button>
    {/if}
    {#if task.status === 'review'}
      <button class="nes-btn is-success btn-sm" on:click={() => onComplete(task)}>Done</button>
    {/if}
    {#if task.type === 'research' && (task.status === 'done' || task.status === 'review')}
      <button class="nes-btn is-warning btn-sm" on:click={() => onViewReport(task)}>Report</button>
    {/if}
    {#if ['assigned', 'in_progress', 'revision'].includes(task.status)}
      <button class="nes-btn btn-sm" on:click={() => onReassign(task)}>Reassign</button>
    {/if}
    {#if ['in_progress', 'revision'].includes(task.status)}
      <button class="nes-btn is-warning btn-sm" on:click={() => onEscalate(task)}>Escalate</button>
    {/if}
    {#if !['done', 'failed'].includes(task.status)}
      <button class="nes-btn is-error btn-sm" on:click={() => onMarkFailed(task)}>Fail</button>
    {/if}
  </div>
</div>

<style>
  .task-card {
    padding: 10px !important;
    margin: 0 0 8px 0 !important;
    cursor: default;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
  }

  .task-title {
    font-size: 10px;
    color: var(--accent-green);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .type-badge {
    font-size: 7px;
    padding: 1px 4px;
    border: 1px solid;
    font-weight: bold;
  }

  .type-badge.research {
    color: #f0c040;
    border-color: #f0c040;
  }

  .type-badge.admin {
    color: #3498db;
    border-color: #3498db;
  }

  .type-badge.hr {
    color: #9b59b6;
    border-color: #9b59b6;
  }

  .type-badge.rejection {
    color: #e74c3c;
    border-color: #e74c3c;
  }

  .task-desc {
    font-size: 8px;
    color: var(--text-secondary);
    margin: 4px 0;
  }

  .assignee, .milestone, .deps {
    font-size: 8px;
    margin: 3px 0;
  }

  .label {
    color: var(--text-secondary);
  }

  .card-actions {
    margin-top: 6px;
    display: flex;
    gap: 4px;
  }

  .btn-sm {
    font-size: 8px !important;
    padding: 2px 8px !important;
  }
</style>
