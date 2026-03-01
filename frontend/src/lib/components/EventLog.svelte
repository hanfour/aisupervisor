<script>
  import { events } from '../stores/events.js'
  import { companyEvents } from '../stores/company.js'

  // Merge supervisor + company events, sorted by time descending
  $: allEvents = mergeEvents($events, $companyEvents)

  function mergeEvents(supEvents, coEvents) {
    const tagged = [
      ...supEvents.map(e => ({ ...e, _source: 'supervisor' })),
      ...coEvents.map(e => ({ ...e, _source: 'company' })),
    ]
    tagged.sort((a, b) => {
      const ta = new Date(a.timestamp).getTime() || 0
      const tb = new Date(b.timestamp).getTime() || 0
      return tb - ta
    })
    return tagged.slice(0, 200)
  }

  function typeLabel(type, source) {
    if (source === 'company') {
      const labels = {
        'project_created': 'PROJ+',
        'task_created': 'TASK+',
        'task_assigned': 'ASSIGN',
        'task_completed': 'DONE',
        'task_failed': 'FAIL',
        'worker_spawned': 'HIRE',
        'worker_idle': 'IDLE',
        'branch_created': 'BRANCH',
        'commit_detected': 'COMMIT',
        'auto_assigned': 'AUTO',
      }
      return labels[type] || type
    }
    const labels = {
      'detected': 'DETECT',
      'decision': 'DECIDE',
      'auto_approved': 'AUTO',
      'sent': 'SENT',
      'paused': 'PAUSE',
      'error': 'ERROR',
      'role_intervention': 'ROLE',
      'role_observation': 'OBS',
    }
    return labels[type] || type
  }

  function typeClass(type, source) {
    if (source === 'company') {
      switch (type) {
        case 'task_completed':
        case 'auto_assigned': return 'status-active'
        case 'task_failed': return 'status-error'
        case 'worker_idle': return 'status-paused'
        default: return ''
      }
    }
    switch (type) {
      case 'auto_approved':
      case 'sent': return 'status-active'
      case 'paused': return 'status-paused'
      case 'error': return 'status-error'
      default: return ''
    }
  }

  function formatTime(ts) {
    try {
      return new Date(ts).toLocaleTimeString('en-US', { hour12: false })
    } catch {
      return ts
    }
  }

  function detailText(event) {
    if (event._source === 'company') {
      return event.message || ''
    }
    let parts = []
    if (event.summary) parts.push(event.summary)
    if (event.reasoning) parts.push('— ' + event.reasoning)
    if (event.error) parts.push(event.error)
    return parts.join(' ')
  }

  function sessionText(event) {
    if (event._source === 'company') return event.workerId || event.projectId || ''
    return event.sessionName || event.sessionId || ''
  }
</script>

<div class="event-log">
  <table class="nes-table is-bordered is-dark">
    <thead>
      <tr>
        <th>Time</th>
        <th>Type</th>
        <th>Source</th>
        <th>Detail</th>
      </tr>
    </thead>
    <tbody>
      {#each allEvents as event, i}
        <tr class="event-new">
          <td class="time">{formatTime(event.timestamp)}</td>
          <td class={typeClass(event.type, event._source)}>
            {typeLabel(event.type, event._source)}
          </td>
          <td>{sessionText(event)}</td>
          <td class="detail">{detailText(event)}</td>
        </tr>
      {/each}
      {#if allEvents.length === 0}
        <tr>
          <td colspan="4" class="empty">Waiting for events...</td>
        </tr>
      {/if}
    </tbody>
  </table>
</div>

<style>
  .event-log {
    overflow-y: auto;
    max-height: 100%;
  }

  table {
    width: 100%;
    font-size: 9px;
  }

  th {
    font-size: 9px;
    text-align: left;
    padding: 6px 8px !important;
  }

  td {
    padding: 4px 8px !important;
    vertical-align: top;
  }

  .time {
    white-space: nowrap;
    color: var(--text-secondary);
  }

  .detail {
    max-width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty {
    text-align: center;
    color: var(--text-secondary);
    padding: 20px !important;
  }
</style>
