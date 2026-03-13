<script>
  import { onMount } from 'svelte'
  import { sessions } from '../stores/sessions.js'
  import TerminalCard from '../components/TerminalCard.svelte'
  import EventLog from '../components/EventLog.svelte'
  import ConfirmDialog from '../components/ConfirmDialog.svelte'
  import ReviewQueuePanel from '../components/ReviewQueuePanel.svelte'
  import TrainingStatsPanel from '../components/TrainingStatsPanel.svelte'
  import { events } from '../stores/events.js'
  import { companyStats, loadCompanyStats, loadReviewQueue, loadTrainingStats, dashboardAlerts, loadDashboardAlerts, budgetSummary, loadBudgetSummary, objectivesList, loadObjectivesList } from '../stores/company.js'
  import { t } from '../stores/i18n.js'

  export let onNavigate = () => {}

  let confirmEvent = null
  let showConfirm = false

  // Watch for paused events that need human confirmation
  $: {
    const paused = $events.find(e => e.type === 'paused' && !e._handled)
    if (paused) {
      confirmEvent = paused
      showConfirm = true
    }
  }

  onMount(() => {
    loadCompanyStats()
    loadReviewQueue()
    loadTrainingStats()
    loadDashboardAlerts()
    loadBudgetSummary()
    loadObjectivesList()
  })

  function handleApprove() {
    if (confirmEvent && window.go?.gui?.App) {
      window.go.gui.App.ApproveEvent(confirmEvent.sessionId, confirmEvent.chosenKey)
    }
    confirmEvent._handled = true
    showConfirm = false
    confirmEvent = null
  }

  function handleDismiss() {
    if (confirmEvent) confirmEvent._handled = true
    showConfirm = false
    confirmEvent = null
  }
</script>

<div class="dashboard">
  {#if $dashboardAlerts.stuckWorkers > 0 || $dashboardAlerts.escalatedTasks > 0 || $dashboardAlerts.pendingApprovals > 0}
  <div class="alert-banner">
    {#if $dashboardAlerts.stuckWorkers > 0}
      <button class="alert-item alert-red" on:click={() => onNavigate('workers')}>
        {$t('alerts.stuckWorkers').replace('{n}', $dashboardAlerts.stuckWorkers)}
      </button>
    {/if}
    {#if $dashboardAlerts.escalatedTasks > 0}
      <button class="alert-item alert-yellow" on:click={() => onNavigate('board')}>
        {$t('alerts.escalatedTasks').replace('{n}', $dashboardAlerts.escalatedTasks)}
      </button>
    {/if}
    {#if $dashboardAlerts.pendingApprovals > 0}
      <button class="alert-item alert-yellow" on:click={() => onNavigate('approvals')}>
        {$t('alerts.pendingApprovals').replace('{n}', $dashboardAlerts.pendingApprovals)}
      </button>
    {/if}
  </div>
  {/if}

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('dashboard.company')}</p>
    <div class="stat-cards">
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{$companyStats.projects}</span>
        <span class="stat-label">{$t('dashboard.projects')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{$companyStats.inProgress}</span>
        <span class="stat-label">{$t('dashboard.inProgress')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{$companyStats.idleWorkers}</span>
        <span class="stat-label">{$t('dashboard.idleWorkers')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{$companyStats.reviewsPending}</span>
        <span class="stat-label">{$t('dashboard.reviewsPending')}</span>
      </div>
      <div class="nes-container is-rounded stat-card">
        <span class="stat-value">{$companyStats.trainingPairs}</span>
        <span class="stat-label">{$t('dashboard.trainingPairs')}</span>
      </div>
    </div>
  </section>

  {#if $budgetSummary.tokensUsed > 0 || $budgetSummary.tokenBudget > 0}
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('dashboard.budget')}</p>
    <div class="budget-row">
      <span class="budget-text">{$t('dashboard.tokensUsed')}: {($budgetSummary.tokensUsed || 0).toLocaleString()}</span>
      {#if $budgetSummary.tokenBudget > 0}
        <span class="budget-text">/ {$budgetSummary.tokenBudget.toLocaleString()}</span>
        <div class="budget-bar-wrap">
          <div class="budget-bar" style="width: {$budgetSummary.usagePercent || 0}%"
               class:warn={$budgetSummary.usagePercent > 80}></div>
        </div>
      {/if}
    </div>
  </section>
  {/if}

  {#if $objectivesList.length > 0}
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('dashboard.objectiveProgress')}</p>
    <div class="obj-overview">
      {#each $objectivesList.filter(o => o.status === 'active').slice(0, 3) as obj}
        <div class="obj-mini">
          <span class="obj-mini-title">{obj.title}</span>
          <span class="obj-mini-projects">{(obj.projectIds || []).length} {$t('boardOverview.projects')}</span>
        </div>
      {/each}
    </div>
  </section>
  {/if}

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('dashboard.reviewQueue')}</p>
    <ReviewQueuePanel />
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('dashboard.trainingStats')}</p>
    <TrainingStatsPanel />
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('dashboard.sessions')}</p>
    <div class="sessions-grid">
      {#each $sessions as session}
        <TerminalCard
          {session}
          onClick={() => onNavigate('terminal', session.id)}
        />
      {/each}
      {#if $sessions.length === 0}
        <p class="empty-msg">{$t('dashboard.noSessions')}</p>
      {/if}
    </div>
  </section>

  <section class="nes-container with-title is-dark events-section">
    <p class="title">{$t('dashboard.events')}</p>
    <EventLog />
  </section>

  <ConfirmDialog
    visible={showConfirm}
    event={confirmEvent}
    onApprove={handleApprove}
    onDismiss={handleDismiss}
  />
</div>

<style>
  .dashboard {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
    overflow: hidden;
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
    font-size: 20px;
    color: var(--accent-green);
    font-weight: bold;
  }

  .stat-label {
    font-size: 9px;
    color: var(--text-secondary);
    margin-top: 4px;
  }

  .sessions-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
  }

  .events-section {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
  }

  .alert-banner {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .alert-item {
    font-size: 9px;
    padding: 6px 12px;
    border: 2px solid;
    cursor: pointer;
    background: transparent;
    font-family: inherit;
  }

  .alert-red {
    border-color: #e74c3c;
    color: #e74c3c;
  }

  .alert-red:hover {
    background: rgba(231, 76, 60, 0.1);
  }

  .alert-yellow {
    border-color: #f0c040;
    color: #f0c040;
  }

  .alert-yellow:hover {
    background: rgba(240, 192, 64, 0.1);
  }

  .budget-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .budget-text {
    font-size: 10px;
  }

  .budget-bar-wrap {
    flex: 1;
    min-width: 80px;
    height: 10px;
    background: rgba(255,255,255,0.1);
    border: 1px solid var(--border-color);
  }

  .budget-bar {
    height: 100%;
    background: var(--accent-green);
    transition: width 0.3s;
  }

  .budget-bar.warn {
    background: #e74c3c;
  }

  .obj-overview {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }

  .obj-mini {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px 10px;
    border: 1px solid var(--border-color);
    min-width: 120px;
  }

  .obj-mini-title {
    font-size: 10px;
    color: var(--accent-green);
  }

  .obj-mini-projects {
    font-size: 8px;
    color: var(--text-secondary);
  }
</style>
