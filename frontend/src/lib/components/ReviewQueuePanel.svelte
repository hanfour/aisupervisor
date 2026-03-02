<script>
  import { reviewQueue, loadReviewQueue } from '../stores/company.js'
  import { onMount } from 'svelte'

  onMount(() => {
    loadReviewQueue()
  })

  function formatDate(ts) {
    if (!ts) return '—'
    const d = new Date(ts)
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
</script>

<div class="review-queue">
  {#if $reviewQueue.length === 0}
    <p class="empty-msg">No pending reviews</p>
  {:else}
    <div class="table-wrap">
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>Task</th>
            <th>Project</th>
            <th>Engineer</th>
            <th>Manager</th>
            <th>Created</th>
          </tr>
        </thead>
        <tbody>
          {#each $reviewQueue as item}
            <tr>
              <td>{item.taskTitle || item.taskId || '—'}</td>
              <td>{item.projectName || item.projectId || '—'}</td>
              <td>{item.engineerName || item.engineerId || '—'}</td>
              <td>{item.managerName || item.managerId || '—'}</td>
              <td>{formatDate(item.createdAt)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .review-queue {
    width: 100%;
  }

  .table-wrap {
    overflow-x: auto;
    width: 100%;
  }

  table {
    width: 100%;
    font-size: 9px;
  }

  th, td {
    padding: 6px 8px !important;
    white-space: nowrap;
  }

  th {
    color: var(--accent-blue);
    font-size: 9px;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
    text-align: center;
    padding: 12px;
  }
</style>
