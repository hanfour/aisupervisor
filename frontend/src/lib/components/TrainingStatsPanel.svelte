<script>
  import { trainingStats, loadTrainingStats } from '../stores/company.js'
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'

  onMount(() => {
    loadTrainingStats()
  })

  $: rate = Math.round(($trainingStats.approvalRate || 0) * 100)
  $: rateClass = rate >= 80 ? 'is-success' : rate >= 50 ? 'is-warning' : 'is-error'
</script>

<div class="training-stats">
  <div class="stats-row">
    <div class="stat-item">
      <span class="stat-value">{$trainingStats.totalPairs}</span>
      <span class="stat-label">{$t('training.total')}</span>
    </div>
    <div class="stat-item accepted">
      <span class="stat-value">{$trainingStats.accepted}</span>
      <span class="stat-label">{$t('training.accepted')}</span>
    </div>
    <div class="stat-item rejected">
      <span class="stat-value">{$trainingStats.rejected}</span>
      <span class="stat-label">{$t('training.rejected')}</span>
    </div>
  </div>
  <div class="rate-section">
    <span class="rate-label">{$t('training.approvalRate')}: {rate}%</span>
    <progress class="nes-progress {rateClass}" value={rate} max="100"></progress>
  </div>
</div>

<style>
  .training-stats {
    width: 100%;
  }

  .stats-row {
    display: flex;
    gap: 16px;
    margin-bottom: 12px;
  }

  .stat-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 80px;
  }

  .stat-value {
    font-size: 16px;
    color: var(--accent-green);
    font-weight: bold;
  }

  .stat-item.accepted .stat-value {
    color: var(--accent-green);
  }

  .stat-item.rejected .stat-value {
    color: var(--accent-red, #ff3860);
  }

  .stat-label {
    font-size: 9px;
    color: var(--text-secondary);
    margin-top: 2px;
  }

  .rate-section {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .rate-label {
    font-size: 10px;
    color: var(--text-primary, #fff);
  }

  progress {
    width: 100%;
  }
</style>
