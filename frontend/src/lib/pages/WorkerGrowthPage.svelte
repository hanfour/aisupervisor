<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'
  import { loadSkillTree, skillTree, loadGrowthHistory, growthHistory, submitFeedback, loadFeatureFlags, featureFlags, toggleFlag } from '../stores/growth.js'
  import SkillRadarChart from '../components/SkillRadarChart.svelte'

  export let workerId = ''

  let feedbackText = ''
  let feedbackSending = false
  let feedbackSuccess = false

  const BRANCH_ORDER = ['frontend', 'backend', 'security', 'devops', 'architecture', 'research']

  async function load() {
    if (!workerId) return
    await Promise.all([
      loadSkillTree(workerId),
      loadGrowthHistory(workerId),
      loadFeatureFlags(),
    ])
  }

  async function handleSubmitFeedback() {
    if (!feedbackText.trim() || feedbackSending) return
    feedbackSending = true
    feedbackSuccess = false
    try {
      await submitFeedback(workerId, '', feedbackText)
      feedbackText = ''
      feedbackSuccess = true
      await loadSkillTree(workerId)
      setTimeout(() => feedbackSuccess = false, 2000)
    } catch (e) {
      console.error('Feedback error:', e)
    }
    feedbackSending = false
  }

  onMount(load)
  $: if (workerId) load()
</script>

<div class="growth-page">
  <h2 class="page-title">{$t('growth.title')}</h2>

  <div class="growth-layout">
    <!-- Left column: Radar + Branch progress -->
    <div class="growth-col">
      <section class="nes-container is-dark growth-section">
        <h3 class="section-title">{$t('growth.skillTree')}</h3>
        {#if $skillTree && $skillTree.branches}
          <div class="radar-wrapper">
            <SkillRadarChart branches={$skillTree.branches} width={320} height={320} />
          </div>
          <div class="summary-row">
            <span class="summary-label">{$t('growth.dominant')}</span>
            <span class="summary-value">{$t('growth.branch.' + ($skillTree.dominantBranch || 'backend'))}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">{$t('growth.avgLevel')}</span>
            <span class="summary-value">{($skillTree.averageLevel || 1).toFixed(1)}</span>
          </div>
        {:else}
          <p class="empty-text">{$t('growth.noEvents')}</p>
        {/if}
      </section>

      <!-- Branch progress bars -->
      <section class="nes-container is-dark growth-section">
        <h3 class="section-title">{$t('growth.level')}</h3>
        {#if $skillTree && $skillTree.branches}
          {#each BRANCH_ORDER as branch}
            {@const node = $skillTree.branches[branch]}
            {#if node}
              <div class="branch-row">
                <span class="branch-name">{$t('growth.branch.' + branch)}</span>
                <span class="branch-level">Lv{node.level}</span>
                <div class="branch-bar-wrap">
                  <progress
                    class="nes-progress is-primary"
                    value={node.currentExp}
                    max={Math.max(1, node.currentExp + node.expToNext)}
                    style="height: 8px;"
                  ></progress>
                </div>
                <span class="branch-exp">{node.currentExp}/{node.currentExp + node.expToNext}</span>
              </div>
            {/if}
          {/each}
        {/if}
      </section>
    </div>

    <!-- Right column: History + Feedback + Feature flags -->
    <div class="growth-col">
      <!-- Growth timeline -->
      <section class="nes-container is-dark growth-section">
        <h3 class="section-title">{$t('growth.history')}</h3>
        {#if $growthHistory && $growthHistory.length > 0}
          <div class="timeline">
            {#each $growthHistory as event}
              <div class="timeline-entry">
                <span class="timeline-date">[{new Date(event.date).toLocaleDateString()}]</span>
                <span class="timeline-text">{event.description || event.type}</span>
                {#if event.expGain}
                  <span class="timeline-exp">+{event.expGain} EXP</span>
                {/if}
              </div>
            {/each}
          </div>
        {:else}
          <p class="empty-text">{$t('growth.noEvents')}</p>
        {/if}
      </section>

      <!-- Boss feedback -->
      <section class="nes-container is-dark growth-section">
        <h3 class="section-title">{$t('growth.feedback')}</h3>
        <textarea
          class="nes-textarea feedback-input"
          bind:value={feedbackText}
          placeholder={$t('growth.feedbackPlaceholder')}
          rows="3"
        ></textarea>
        <div class="feedback-actions">
          <button
            class="nes-btn is-primary btn-sm"
            on:click={handleSubmitFeedback}
            disabled={feedbackSending || !feedbackText.trim()}
          >
            {feedbackSending ? '...' : $t('growth.submitFeedback')}
          </button>
          {#if feedbackSuccess}
            <span class="feedback-ok">OK</span>
          {/if}
        </div>
      </section>

      <!-- Feature flags -->
      <section class="nes-container is-dark growth-section">
        <h3 class="section-title">{$t('growth.featureFlags')}</h3>
        {#if $featureFlags && $featureFlags.length > 0}
          {#each $featureFlags as flag}
            <div class="flag-row">
              <label>
                <input
                  type="checkbox"
                  class="nes-checkbox is-dark"
                  checked={flag.enabled}
                  on:change={() => toggleFlag(flag.id, !flag.enabled)}
                />
                <span>{flag.name}</span>
              </label>
            </div>
          {/each}
        {:else}
          <p class="empty-text">No feature flags</p>
        {/if}
      </section>
    </div>
  </div>
</div>

<style>
  .growth-page {
    padding: 16px;
    max-width: 900px;
    margin: 0 auto;
  }

  .page-title {
    font-size: 14px;
    color: var(--accent-blue, #5bbad5);
    margin-bottom: 16px;
  }

  .growth-layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  @media (max-width: 700px) {
    .growth-layout {
      grid-template-columns: 1fr;
    }
  }

  .growth-col {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .growth-section {
    padding: 12px !important;
  }

  .section-title {
    font-size: 10px;
    color: var(--accent-green, #6bb87b);
    margin-bottom: 8px;
  }

  .radar-wrapper {
    display: flex;
    justify-content: center;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    font-size: 9px;
    padding: 2px 0;
  }

  .summary-label {
    color: var(--text-secondary, #999);
  }

  .summary-value {
    color: var(--accent-blue, #5bbad5);
  }

  .branch-row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 4px 0;
    font-size: 8px;
  }

  .branch-name {
    width: 40px;
    color: var(--text-secondary, #999);
  }

  .branch-level {
    width: 28px;
    color: var(--accent-yellow, #e8a855);
    font-weight: bold;
  }

  .branch-bar-wrap {
    flex: 1;
  }

  .branch-exp {
    width: 60px;
    text-align: right;
    color: var(--text-secondary, #999);
  }

  .empty-text {
    font-size: 9px;
    color: var(--text-secondary, #999);
    text-align: center;
    padding: 12px 0;
  }

  .timeline {
    max-height: 200px;
    overflow-y: auto;
    font-size: 8px;
  }

  .timeline-entry {
    padding: 4px 0;
    border-bottom: 1px solid rgba(255,255,255,0.05);
    display: flex;
    gap: 6px;
    align-items: baseline;
  }

  .timeline-date {
    color: var(--accent-blue, #5bbad5);
    white-space: nowrap;
  }

  .timeline-text {
    flex: 1;
  }

  .timeline-exp {
    color: var(--accent-green, #6bb87b);
    white-space: nowrap;
  }

  .feedback-input {
    width: 100%;
    font-size: 9px;
    resize: vertical;
    background: var(--bg-primary, #0d0d1a) !important;
    color: var(--text, #e0e0e0) !important;
  }

  .feedback-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
  }

  .feedback-ok {
    font-size: 9px;
    color: var(--accent-green, #6bb87b);
  }

  .btn-sm {
    font-size: 8px !important;
    padding: 4px 8px !important;
  }

  .flag-row {
    font-size: 9px;
    padding: 4px 0;
  }

  .flag-row label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
  }
</style>
