<script>
  import { onMount } from 'svelte'
  import { hierarchy, loadHierarchy, getSubordinates } from '../stores/workers.js'
  import { addError } from '../stores/errors.js'
  import WorkerCard from '../components/WorkerCard.svelte'
  import WorkerDetailDrawer from '../components/WorkerDetailDrawer.svelte'
  import ReviewQueuePanel from '../components/ReviewQueuePanel.svelte'
  import TrainingStatsPanel from '../components/TrainingStatsPanel.svelte'

  let selectedWorkerId = null
  let expandedWorkers = {}

  const tierOrder = ['consultant', 'manager', 'engineer']
  const tierColors = {
    consultant: 'var(--accent-yellow, #ffdd57)',
    manager: 'var(--accent-blue)',
    engineer: 'var(--accent-green)',
  }

  function tierLabel(tier) {
    return tier.charAt(0).toUpperCase() + tier.slice(1) + 's'
  }

  async function toggleExpand(workerId) {
    if (expandedWorkers[workerId]) {
      expandedWorkers = { ...expandedWorkers, [workerId]: null }
    } else {
      try {
        const subs = await getSubordinates(workerId)
        expandedWorkers = { ...expandedWorkers, [workerId]: subs }
      } catch (e) {
        addError('Failed to load subordinates: ' + e.message)
      }
    }
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
                <div class="card-row">
                  <WorkerCard worker={w} onClick={(worker) => selectedWorkerId = worker.id} />
                  {#if tier !== 'engineer'}
                    <button
                      class="nes-btn expand-btn"
                      on:click|stopPropagation={() => toggleExpand(w.id)}
                      title={expandedWorkers[w.id] ? 'Collapse' : 'Expand subordinates'}
                    >
                      {expandedWorkers[w.id] ? '−' : '+'}
                    </button>
                  {/if}
                </div>
                <div class="card-meta">
                  {#if w.tier}
                    <span class="meta-tier" style="color: {tierColors[w.tier]}">[{w.tier}]</span>
                  {/if}
                  {#if w.parentName}
                    <span class="meta-parent">&uarr; {w.parentName}</span>
                  {/if}
                </div>
                {#if expandedWorkers[w.id]}
                  <div class="sub-tree">
                    {#each expandedWorkers[w.id] as sub}
                      <div class="sub-row">
                        <span class="tree-connector">&boxur;&HorizontalLine;</span>
                        <button class="nes-btn link-btn" on:click={() => selectedWorkerId = sub.id}>
                          {sub.name}
                          <span class="sub-tier" style="color: {tierColors[sub.tier]}">[{sub.tier}]</span>
                          <span class="sub-status">{sub.status}</span>
                        </button>
                      </div>
                    {/each}
                    {#if expandedWorkers[w.id].length === 0}
                      <span class="empty-sub">No subordinates</span>
                    {/if}
                  </div>
                {/if}
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

  {#if selectedWorkerId}
    <WorkerDetailDrawer
      workerId={selectedWorkerId}
      onClose={() => selectedWorkerId = null}
      onSelectWorker={(id) => selectedWorkerId = id}
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

  .card-row {
    display: flex;
    align-items: flex-start;
    gap: 4px;
  }

  .expand-btn {
    font-size: 10px !important;
    padding: 2px 6px !important;
    min-width: 24px;
    line-height: 1;
    flex-shrink: 0;
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

  .sub-tree {
    margin-left: 16px;
    padding-left: 8px;
    border-left: 2px solid var(--border-color);
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .sub-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .tree-connector {
    color: var(--text-secondary);
    font-size: 12px;
    flex-shrink: 0;
  }

  .link-btn {
    font-size: 8px !important;
    padding: 2px 6px !important;
    text-align: left;
  }

  .sub-tier {
    font-weight: bold;
  }

  .sub-status {
    color: var(--text-secondary);
    font-size: 8px;
  }

  .empty-sub {
    font-size: 8px;
    color: var(--text-secondary);
    padding: 4px 0;
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
