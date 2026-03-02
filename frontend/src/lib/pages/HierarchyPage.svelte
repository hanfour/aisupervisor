<script>
  import { onMount } from 'svelte'
  import { hierarchy, loadHierarchy } from '../stores/workers.js'
  import { addError } from '../stores/errors.js'
  import WorkerCard from '../components/WorkerCard.svelte'
  import WorkerLogPanel from '../components/WorkerLogPanel.svelte'
  import ReviewQueuePanel from '../components/ReviewQueuePanel.svelte'
  import TrainingStatsPanel from '../components/TrainingStatsPanel.svelte'

  let logWorker = null

  const tierOrder = ['consultant', 'manager', 'engineer']
  const tierColors = {
    consultant: 'var(--accent-yellow, #ffdd57)',
    manager: 'var(--accent-blue)',
    engineer: 'var(--accent-green)',
  }

  function tierLabel(tier) {
    return tier.charAt(0).toUpperCase() + tier.slice(1) + 's'
  }

  onMount(async () => {
    try {
      await loadHierarchy()
    } catch (e) {
      addError('Failed to load hierarchy: ' + e.message)
    }
  })
</script>

<div class="hierarchy-page">
  <section class="nes-container with-title is-dark">
    <p class="title">Company Hierarchy</p>

    <div class="hierarchy-columns">
      {#each tierOrder as tier}
        {@const tierWorkers = $hierarchy[tier] || []}
        <div class="tier-col">
          <div class="tier-badge" style="border-color: {tierColors[tier]}">
            <span class="tier-icon">
              {#if tier === 'consultant'}&#9733;{:else if tier === 'manager'}&#9830;{:else}&#9881;{/if}
            </span>
            <span class="tier-name" style="color: {tierColors[tier]}">{tierLabel(tier)}</span>
            <span class="tier-count">({tierWorkers.length})</span>
          </div>

          <div class="tier-list">
            {#each tierWorkers as w}
              <div class="hierarchy-card">
                <WorkerCard worker={w} onClick={(worker) => logWorker = worker} />
                <div class="card-meta">
                  {#if w.tier}
                    <span class="meta-tier" style="color: {tierColors[w.tier]}">[{w.tier}]</span>
                  {/if}
                  {#if w.parentName}
                    <span class="meta-parent">&uarr; {w.parentName}</span>
                  {/if}
                </div>
              </div>
            {/each}
            {#if tierWorkers.length === 0}
              <p class="empty-msg">No {tier}s hired yet</p>
            {/if}
          </div>
        </div>

        {#if tier !== 'engineer'}
          <div class="tier-arrow">&rarr;</div>
        {/if}
      {/each}
    </div>
  </section>

  <div class="bottom-panels">
    <section class="nes-container with-title is-dark panel-half">
      <p class="title">Review Queue</p>
      <ReviewQueuePanel />
    </section>
    <section class="nes-container with-title is-dark panel-half">
      <p class="title">Training Stats</p>
      <TrainingStatsPanel />
    </section>
  </div>

  {#if logWorker}
    <WorkerLogPanel
      workerId={logWorker.id}
      workerName={logWorker.name}
      onClose={() => logWorker = null}
    />
  {/if}
</div>

<style>
  .hierarchy-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
  }

  .hierarchy-columns {
    display: flex;
    align-items: flex-start;
    gap: 8px;
  }

  .tier-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .tier-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    border: 2px solid;
    border-radius: 4px;
  }

  .tier-icon {
    font-size: 14px;
  }

  .tier-name {
    font-size: 11px;
    font-weight: bold;
  }

  .tier-count {
    font-size: 10px;
    color: var(--text-secondary);
  }

  .tier-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .tier-arrow {
    display: flex;
    align-items: center;
    font-size: 20px;
    color: var(--text-secondary);
    padding-top: 30px;
  }

  .hierarchy-card {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .card-meta {
    display: flex;
    gap: 8px;
    justify-content: center;
    font-size: 9px;
  }

  .meta-tier {
    font-weight: bold;
  }

  .meta-parent {
    color: var(--text-secondary);
  }

  .bottom-panels {
    display: flex;
    gap: 12px;
  }

  .panel-half {
    flex: 1;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
    text-align: center;
    padding: 12px;
  }
</style>
