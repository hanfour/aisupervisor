<script>
  import { sessions } from '../stores/sessions.js'
  import { events } from '../stores/events.js'
  import { roles } from '../stores/roles.js'

  export let sessionId = ''
  export let onBack = () => {}

  $: session = $sessions.find(s => s.id === sessionId) || {}
  $: sessionEvents = $events.filter(e => e.sessionId === sessionId)
  $: sessionRoleIds = []

  async function loadSessionRoles() {
    if (window.go?.gui?.App) {
      sessionRoleIds = (await window.go.gui.App.GetSessionRoles(sessionId)) || []
    }
  }

  $: if (sessionId) loadSessionRoles()

  function typeLabel(type) {
    const labels = {
      'detected': 'DETECT',
      'decision': 'DECIDE',
      'auto_approved': 'AUTO',
      'sent': 'SENT',
      'paused': 'PAUSE',
      'error': 'ERROR',
      'role_intervention': 'ROLE',
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

<div class="terminal-page">
  <div class="header">
    <button class="nes-btn is-primary" on:click={onBack}>&lt; Back</button>
    <h2>{session.name || sessionId}</h2>
    <span class="nes-badge">
      <span class={session.status === 'active' ? 'is-success' : 'is-warning'}>
        {session.status || 'unknown'}
      </span>
    </span>
  </div>

  <div class="content">
    <section class="nes-container with-title is-dark info-panel">
      <p class="title">Info</p>
      <div class="info-grid">
        <div><span class="label">tmux:</span> {session.tmuxSession}:{session.window}.{session.pane}</div>
        <div><span class="label">tool:</span> {session.toolType || 'auto'}</div>
        {#if session.projectDir}
          <div><span class="label">dir:</span> {session.projectDir}</div>
        {/if}
        {#if session.taskGoal}
          <div><span class="label">goal:</span> {session.taskGoal}</div>
        {/if}
      </div>

      {#if sessionRoleIds.length > 0}
        <div class="roles-section">
          <span class="label">Assigned roles:</span>
          {#each sessionRoleIds as rid}
            <span class="nes-badge"><span class="is-primary">{rid}</span></span>
          {/each}
        </div>
      {/if}
    </section>

    <section class="nes-container with-title is-dark events-panel">
      <p class="title">Session Events</p>
      <div class="event-list">
        {#each sessionEvents as event}
          <div class="event-row event-new">
            <span class="time">{formatTime(event.timestamp)}</span>
            <span class={typeClass(event.type)}>{typeLabel(event.type)}</span>
            {#if event.summary}<span>{event.summary}</span>{/if}
            {#if event.reasoning}<span class="reasoning">— {event.reasoning}</span>{/if}
            {#if event.roleId}<span class="role-tag">[{event.roleId}]</span>{/if}
          </div>
        {/each}
        {#if sessionEvents.length === 0}
          <p class="empty-msg">No events for this session</p>
        {/if}
      </div>
    </section>
  </div>
</div>

<style>
  .terminal-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .header {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 16px;
  }

  h2 {
    font-size: 14px;
    color: var(--accent-green);
    margin: 0;
  }

  .content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-height: 0;
    overflow: hidden;
  }

  .info-panel {
    font-size: 10px;
  }

  .info-grid {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .label {
    color: var(--text-secondary);
  }

  .roles-section {
    margin-top: 8px;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }

  .events-panel {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .event-list {
    overflow-y: auto;
    flex: 1;
    font-size: 9px;
  }

  .event-row {
    padding: 4px 0;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .time {
    color: var(--text-secondary);
  }

  .reasoning {
    color: var(--text-secondary);
  }

  .role-tag {
    color: var(--accent-blue);
  }

  .empty-msg {
    color: var(--text-secondary);
    text-align: center;
    padding: 20px;
  }
</style>
