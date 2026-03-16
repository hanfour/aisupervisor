<script>
  import { onMount, onDestroy } from 'svelte'
  import { t } from '../stores/i18n.js'

  export let workers = []

  let activities = {}
  let intervalId

  async function fetchActivity(workerID) {
    if (!window.go?.gui?.CompanyApp) return ''
    try {
      return await window.go.gui.CompanyApp.GetWorkerActivity(workerID)
    } catch {
      return ''
    }
  }

  async function refreshAll() {
    const working = workers.filter(w => w.status === 'working' || w.status === 'waiting')
    for (const w of working) {
      const output = await fetchActivity(w.id)
      if (output) {
        // Extract last 3 non-empty lines
        const lines = output.split('\n').filter(l => l.trim()).slice(-3)
        activities[w.id] = lines.join('\n')
      }
    }
    activities = { ...activities }
  }

  onMount(() => {
    refreshAll()
    intervalId = setInterval(refreshAll, 5000)
  })

  onDestroy(() => {
    if (intervalId) clearInterval(intervalId)
  })
</script>

<div class="activity-feed">
  <h3>{$t('activity.title')}</h3>
  {#each workers.filter(w => w.status === 'working' || w.status === 'waiting') as w (w.id)}
    <div class="activity-item">
      <div class="activity-header">
        <span class="worker-name">{w.name}</span>
        <span class="worker-status status-{w.status}">{w.status}</span>
      </div>
      {#if activities[w.id]}
        <pre class="activity-output">{activities[w.id]}</pre>
      {:else}
        <span class="no-activity">{$t('activity.noOutput')}</span>
      {/if}
    </div>
  {:else}
    <p class="empty">{$t('activity.noActive')}</p>
  {/each}
</div>

<style>
  .activity-feed {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  h3 {
    margin: 0 0 0.5rem 0;
    font-size: 0.9rem;
    opacity: 0.8;
  }
  .activity-item {
    background: var(--bg-secondary, #1a1a2e);
    border-radius: 6px;
    padding: 0.5rem 0.75rem;
    font-size: 0.8rem;
  }
  .activity-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.25rem;
  }
  .worker-name {
    font-weight: 600;
  }
  .worker-status {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    background: var(--accent, #6366f1);
    color: white;
  }
  .status-working { background: #22c55e; }
  .status-waiting { background: #f59e0b; }
  .activity-output {
    margin: 0;
    font-family: 'SF Mono', monospace;
    font-size: 0.7rem;
    line-height: 1.3;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 4rem;
    overflow: hidden;
    opacity: 0.7;
  }
  .no-activity, .empty {
    opacity: 0.5;
    font-size: 0.75rem;
  }
</style>
