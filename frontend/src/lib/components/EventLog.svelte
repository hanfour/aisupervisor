<script>
  import { events } from '../stores/events.js'

  function typeLabel(type) {
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

  function typeClass(type) {
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
</script>

<div class="event-log">
  <table class="nes-table is-bordered is-dark">
    <thead>
      <tr>
        <th>Time</th>
        <th>Type</th>
        <th>Session</th>
        <th>Detail</th>
      </tr>
    </thead>
    <tbody>
      {#each $events as event, i}
        <tr class="event-new">
          <td class="time">{formatTime(event.timestamp)}</td>
          <td class={typeClass(event.type)}>
            {typeLabel(event.type)}
          </td>
          <td>{event.sessionName || event.sessionId}</td>
          <td class="detail">
            {#if event.summary}
              {event.summary}
            {/if}
            {#if event.reasoning}
              — {event.reasoning}
            {/if}
            {#if event.error}
              <span class="status-error">{event.error}</span>
            {/if}
          </td>
        </tr>
      {/each}
      {#if $events.length === 0}
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
