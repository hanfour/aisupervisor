<script>
  import DiscussionMessage from './DiscussionMessage.svelte'

  export let discussion = { id: '', events: [], latestPhase: 'opinion' }

  $: opinionEvents = discussion.events.filter(e => e.phase === 'opinion')
  $: roundtableEvents = discussion.events.filter(e => e.phase === 'roundtable')
  $: decisionEvents = discussion.events.filter(e => e.phase === 'decision')

  function phaseStatus(phase) {
    if (phase === discussion.latestPhase) return 'active'
    const order = ['opinion', 'roundtable', 'decision']
    return order.indexOf(phase) < order.indexOf(discussion.latestPhase) ? 'done' : 'pending'
  }
</script>

<div class="discussion">
  <div class="phase-indicator">
    <span class="phase" class:done={phaseStatus('opinion') === 'done'} class:active={phaseStatus('opinion') === 'active'}>
      1. Opinions
    </span>
    <span class="arrow">→</span>
    <span class="phase" class:done={phaseStatus('roundtable') === 'done'} class:active={phaseStatus('roundtable') === 'active'}>
      2. Roundtable
    </span>
    <span class="arrow">→</span>
    <span class="phase" class:done={phaseStatus('decision') === 'done'} class:active={phaseStatus('decision') === 'active'}>
      3. Decision
    </span>
  </div>

  {#if opinionEvents.length > 0}
    <div class="phase-section">
      <h4>Opinions ({opinionEvents.length})</h4>
      <div class="opinions-grid">
        {#each opinionEvents as event}
          <DiscussionMessage {event} />
        {/each}
      </div>
    </div>
  {/if}

  {#if roundtableEvents.length > 0}
    <div class="phase-section">
      <h4>Roundtable Discussion</h4>
      {#each roundtableEvents as event}
        <DiscussionMessage {event} />
      {/each}
    </div>
  {/if}

  {#if decisionEvents.length > 0}
    <div class="phase-section">
      <h4>Final Decision</h4>
      {#each decisionEvents as event}
        <DiscussionMessage {event} isLeader={true} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .discussion {
    padding: 8px;
  }

  .phase-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
    font-size: 10px;
  }

  .phase {
    padding: 4px 8px;
    border: 2px solid var(--border-color);
    color: var(--text-secondary);
  }

  .phase.active {
    border-color: var(--accent-yellow);
    color: var(--accent-yellow);
  }

  .phase.done {
    border-color: var(--accent-green);
    color: var(--accent-green);
  }

  .arrow {
    color: var(--text-secondary);
  }

  .phase-section {
    margin-bottom: 16px;
  }

  h4 {
    font-size: 10px;
    color: var(--accent-blue);
    margin-bottom: 8px;
  }

  .opinions-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 12px;
  }
</style>
