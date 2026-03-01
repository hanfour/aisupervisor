<script>
  import BranchStatus from './BranchStatus.svelte'

  export let task = {}
  export let workers = []
  export let onAssign = () => {}
  export let onComplete = () => {}

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
    <span class="task-title">{task.title}</span>
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
