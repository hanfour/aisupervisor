<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'
  import { addError } from '../stores/errors.js'

  let overview = null
  let budget = null
  let performance = []
  let objectives = []

  onMount(loadAll)

  async function loadAll() {
    try {
      overview = await window.go.gui.CompanyApp.GetCompanyOverview()
    } catch (e) {
      addError('Failed to load overview: ' + e.message)
    }
    try {
      budget = await window.go.gui.CompanyApp.GetBudgetSummary()
    } catch {}
    try {
      performance = await window.go.gui.CompanyApp.GetPerformanceHistory('') || []
    } catch {}
    try {
      objectives = await window.go.gui.CompanyApp.ListObjectives() || []
    } catch {}
  }

  // Aggregate performance by worker (latest snapshot per worker)
  $: workerPerf = (() => {
    const map = new Map()
    for (const s of performance) {
      map.set(s.workerId, s)
    }
    return [...map.values()]
  })()

  function miniBar(value, max) {
    if (max <= 0) return 0
    return Math.min((value / max) * 100, 100)
  }
</script>

<div class="board-page">
  {#if overview}
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('boardOverview.title')}</p>
    <div class="stat-cards">
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{overview.activeObjectives}/{overview.totalObjectives}</span>
        <span class="stat-label">{$t('boardOverview.objectives')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{overview.activeProjects}/{overview.totalProjects}</span>
        <span class="stat-label">{$t('boardOverview.projects')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{overview.completedTasks}/{overview.totalTasks}</span>
        <span class="stat-label">{$t('boardOverview.tasks')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{overview.activeWorkers}/{overview.totalWorkers}</span>
        <span class="stat-label">{$t('boardOverview.workers')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{overview.overallApprovalRate ? overview.overallApprovalRate.toFixed(0) + '%' : 'N/A'}</span>
        <span class="stat-label">{$t('boardOverview.approvalRate')}</span>
      </div>
    </div>
  </section>
  {/if}

  {#if budget}
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('boardOverview.budget')}</p>
    <div class="budget-info">
      <div class="budget-row">
        <span>{$t('boardOverview.tokensUsed')}: {(budget.tokensUsed || 0).toLocaleString()}</span>
        {#if budget.tokenBudget > 0}
          <span>/ {budget.tokenBudget.toLocaleString()}</span>
        {/if}
      </div>
      {#if budget.tokenBudget > 0}
        <div class="budget-bar-wrap">
          <div class="budget-bar" style="width: {budget.usagePercent || 0}%"
               class:warn={budget.usagePercent > 80}></div>
        </div>
      {/if}
      <span class="budget-tasks">{$t('boardOverview.tasksThisMonth')}: {budget.taskCount || 0}</span>
    </div>
  </section>
  {/if}

  {#if objectives.length > 0}
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('boardOverview.objectiveProgress')}</p>
    {#each objectives.filter(o => o.status === 'active') as obj}
      <div class="obj-row">
        <span class="obj-name">{obj.title}</span>
        {#if obj.keyResults}
          {#each obj.keyResults as kr}
            <div class="kr-mini">
              <span class="kr-label">{kr.title}</span>
              <div class="kr-bar-wrap">
                <div class="kr-bar" style="width: {kr.target > 0 ? Math.min(kr.current / kr.target * 100, 100) : 0}%"></div>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    {/each}
  </section>
  {/if}

  {#if workerPerf.length > 0}
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('boardOverview.performance')}</p>
    <table class="perf-table">
      <thead>
        <tr>
          <th>{$t('boardOverview.worker')}</th>
          <th>{$t('boardOverview.completed')}</th>
          <th>{$t('boardOverview.failed')}</th>
          <th>{$t('boardOverview.approval')}</th>
          <th>{$t('boardOverview.tokens')}</th>
        </tr>
      </thead>
      <tbody>
        {#each workerPerf as wp}
          <tr>
            <td>{wp.workerId.slice(0, 8)}</td>
            <td>
              <div class="mini-bar-wrap">
                <div class="mini-bar green" style="width: {miniBar(wp.tasksCompleted, 20)}%"></div>
              </div>
              <span class="mini-val">{wp.tasksCompleted}</span>
            </td>
            <td>
              <div class="mini-bar-wrap">
                <div class="mini-bar red" style="width: {miniBar(wp.tasksFailed, 10)}%"></div>
              </div>
              <span class="mini-val">{wp.tasksFailed}</span>
            </td>
            <td>{wp.approvalRate ? wp.approvalRate.toFixed(0) + '%' : 'N/A'}</td>
            <td>{(wp.tokensUsed || 0).toLocaleString()}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
  {/if}
</div>

<style>
  .board-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
    overflow-y: auto;
  }

  .stat-cards {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }

  .stat-card {
    padding: 12px 16px !important;
    margin: 0 !important;
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 100px;
  }

  .stat-value {
    font-size: 18px;
    color: var(--accent-green);
    font-weight: bold;
  }

  .stat-label {
    font-size: 9px;
    color: var(--text-secondary);
    margin-top: 4px;
  }

  .budget-info {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .budget-row {
    font-size: 11px;
  }

  .budget-bar-wrap {
    width: 100%;
    height: 12px;
    background: rgba(255,255,255,0.1);
    border: 2px solid var(--border-color);
  }

  .budget-bar {
    height: 100%;
    background: var(--accent-green);
    transition: width 0.3s;
  }

  .budget-bar.warn {
    background: var(--accent-red, #e76e55);
  }

  .budget-tasks {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .obj-row {
    margin-bottom: 8px;
    padding: 6px 0;
    border-bottom: 1px solid var(--border-color);
  }

  .obj-name {
    font-size: 11px;
    color: var(--accent-green);
    font-weight: bold;
  }

  .kr-mini {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
  }

  .kr-label {
    font-size: 8px;
    flex: 1;
    color: var(--text-secondary);
  }

  .kr-bar-wrap {
    width: 100px;
    height: 6px;
    background: rgba(255,255,255,0.1);
    border: 1px solid var(--border-color);
  }

  .kr-bar {
    height: 100%;
    background: var(--accent-blue);
  }

  .perf-table {
    width: 100%;
    font-size: 9px;
    border-collapse: collapse;
  }

  .perf-table th, .perf-table td {
    padding: 4px 8px;
    text-align: left;
    border-bottom: 1px solid var(--border-color);
  }

  .perf-table th {
    color: var(--accent-blue);
    font-size: 8px;
  }

  .mini-bar-wrap {
    width: 50px;
    height: 6px;
    background: rgba(255,255,255,0.1);
    display: inline-block;
    vertical-align: middle;
    margin-right: 4px;
  }

  .mini-bar {
    height: 100%;
  }

  .mini-bar.green {
    background: var(--accent-green);
  }

  .mini-bar.red {
    background: var(--accent-red, #e76e55);
  }

  .mini-val {
    font-size: 8px;
  }
</style>
