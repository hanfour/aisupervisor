<script>
  import { onMount } from 'svelte'
  import { sessions } from '../stores/sessions.js'
  import TerminalCard from '../components/TerminalCard.svelte'
  import EventLog from '../components/EventLog.svelte'
  import ConfirmDialog from '../components/ConfirmDialog.svelte'
  import ReviewQueuePanel from '../components/ReviewQueuePanel.svelte'
  import TrainingStatsPanel from '../components/TrainingStatsPanel.svelte'
  import { events } from '../stores/events.js'
  import { companyStats, loadCompanyStats, loadReviewQueue, loadTrainingStats } from '../stores/company.js'
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
</style>
