<script>
  import { reviewQueue, loadReviewQueue } from '../stores/company.js'
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'

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
    <p class="empty-msg">{$t('reviewQueue.noReviews')}</p>
  {:else}
    <div class="table-wrap">
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>{$t('reviewQueue.task')}</th>
            <th>{$t('dashboard.projects')}</th>
            <th>{$t('reviewQueue.engineer')}</th>
            <th>{$t('reviewQueue.reviewManager')}</th>
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
