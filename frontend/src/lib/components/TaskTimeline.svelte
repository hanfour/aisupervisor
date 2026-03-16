<script>
  import { t } from '../stores/i18n.js'

  export let task = {}

  const stages = [
    { key: 'created', status: 'ready' },
    { key: 'assigned', status: 'assigned' },
    { key: 'inProgress', status: 'in_progress' },
    { key: 'review', status: 'review' },
    { key: 'done', status: 'done' },
  ]

  // Status progression index
  const statusOrder = {
    backlog: 0,
    ready: 0,
    assigned: 1,
    in_progress: 2,
    code_review: 3,
    review: 3,
    revision: 2, // goes back
    done: 4,
    failed: -1,
  }

  $: currentIndex = statusOrder[task.status] ?? 0
  $: isFailed = task.status === 'failed'
  $: isRevision = task.status === 'revision'

  function formatDuration(ms) {
    if (!ms || ms <= 0) return '-'
    const sec = Math.floor(ms / 1000)
    if (sec < 60) return `${sec}s`
    const min = Math.floor(sec / 60)
    if (min < 60) return `${min}m`
    const hr = Math.floor(min / 60)
    return `${hr}h ${min % 60}m`
  }

  $: createdAt = task.createdAt ? new Date(task.createdAt).getTime() : 0
  $: startedAt = task.startedAt ? new Date(task.startedAt).getTime() : 0
  $: completedAt = task.completedAt ? new Date(task.completedAt).getTime() : 0
  $: waitTime = startedAt && createdAt ? formatDuration(startedAt - createdAt) : '-'
  $: workTime = completedAt && startedAt ? formatDuration(completedAt - startedAt) : '-'
</script>

<div class="timeline" class:failed={isFailed} class:revision={isRevision}>
  {#each stages as stage, i}
    <div class="stage" class:active={i <= currentIndex && !isFailed} class:current={i === currentIndex}>
      <div class="dot" />
      <span class="label">{$t('timeline.' + stage.key)}</span>
    </div>
    {#if i < stages.length - 1}
      <div class="connector" class:filled={i < currentIndex && !isFailed} />
    {/if}
  {/each}
</div>

<div class="durations">
  {#if waitTime !== '-'}
    <span class="duration">{$t('timeline.waitTime')}: {waitTime}</span>
  {/if}
  {#if workTime !== '-'}
    <span class="duration">{$t('timeline.workTime')}: {workTime}</span>
  {/if}
  {#if task.rejectionCount > 0}
    <span class="duration warn">{$t('timeline.rejections')}: {task.rejectionCount}</span>
  {/if}
  {#if task.retryCount > 0}
    <span class="duration warn">{$t('timeline.retries')}: {task.retryCount}</span>
  {/if}
</div>

<style>
  .timeline {
    display: flex;
    align-items: center;
    gap: 0;
    padding: 0.5rem 0;
  }
  .stage {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    min-width: 3rem;
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--border, #333);
    transition: background 0.2s;
  }
  .stage.active .dot {
    background: var(--accent, #6366f1);
  }
  .stage.current .dot {
    background: #22c55e;
    box-shadow: 0 0 6px #22c55e;
  }
  .failed .stage.current .dot {
    background: #ef4444;
    box-shadow: 0 0 6px #ef4444;
  }
  .revision .stage.current .dot {
    background: #f59e0b;
    box-shadow: 0 0 6px #f59e0b;
  }
  .label {
    font-size: 0.65rem;
    opacity: 0.6;
    text-align: center;
  }
  .stage.active .label {
    opacity: 1;
  }
  .connector {
    flex: 1;
    height: 2px;
    background: var(--border, #333);
    min-width: 1rem;
    margin-bottom: 1.2rem;
  }
  .connector.filled {
    background: var(--accent, #6366f1);
  }
  .durations {
    display: flex;
    gap: 0.75rem;
    font-size: 0.7rem;
    opacity: 0.7;
    flex-wrap: wrap;
  }
  .duration.warn {
    color: #f59e0b;
  }
</style>
