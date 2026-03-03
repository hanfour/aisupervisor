<script>
  import { roles } from '../stores/roles.js'
  import { sessions } from '../stores/sessions.js'
  import RoleAssignment from '../components/RoleAssignment.svelte'
  import { t } from '../stores/i18n.js'

  let selectedSessionId = ''
  let sessionRoleMap = {}

  $: if ($sessions.length > 0 && !selectedSessionId) {
    selectedSessionId = $sessions[0]?.id || ''
  }

  async function loadSessionRoles(sid) {
    if (window.go?.gui?.App) {
      const ids = await window.go.gui.App.GetSessionRoles(sid)
      sessionRoleMap[sid] = ids || []
      sessionRoleMap = sessionRoleMap
    }
  }

  $: if (selectedSessionId) loadSessionRoles(selectedSessionId)

  async function handleUpdate(sessionId, roleIds) {
    if (window.go?.gui?.App) {
      await window.go.gui.App.SetSessionRoles(sessionId, roleIds)
      sessionRoleMap[sessionId] = roleIds
      sessionRoleMap = sessionRoleMap
    }
  }

  function modeClass(mode) {
    switch (mode) {
      case 'reactive': return 'is-success'
      case 'proactive': return 'is-warning'
      case 'hybrid': return 'is-primary'
      default: return ''
    }
  }
</script>

<div class="roles-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('roles.allRoles')}</p>
    <table class="nes-table is-bordered is-dark">
      <thead>
        <tr>
          <th>{$t('roles.id')}</th>
          <th>{$t('roles.name')}</th>
          <th>{$t('roles.mode')}</th>
          <th>{$t('roles.priority')}</th>
        </tr>
      </thead>
      <tbody>
        {#each $roles as role}
          <tr>
            <td>{role.id}</td>
            <td>{role.name}</td>
            <td>
              <span class="nes-badge">
                <span class={modeClass(role.mode)}>{role.mode}</span>
              </span>
            </td>
            <td>{role.priority}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>

  <section class="nes-container with-title is-dark assignment-section">
    <p class="title">{$t('roles.perTerminal')}</p>

    {#if $sessions.length > 0}
      <div class="session-selector">
        <label>{$t('roles.session')}</label>
        <div class="nes-select is-dark">
          <select bind:value={selectedSessionId}>
            {#each $sessions as s}
              <option value={s.id}>{s.name || s.id}</option>
            {/each}
          </select>
        </div>
      </div>

      <RoleAssignment
        sessionId={selectedSessionId}
        assignedRoleIds={sessionRoleMap[selectedSessionId] || []}
        onUpdate={handleUpdate}
      />
    {:else}
      <p class="empty">{$t('roles.noSessions')}</p>
    {/if}
  </section>
</div>

<style>
  .roles-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    overflow-y: auto;
    height: 100%;
  }

  table {
    width: 100%;
    font-size: 10px;
  }

  th, td {
    padding: 6px 8px !important;
  }

  .assignment-section {
    flex: 1;
  }

  .session-selector {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
    font-size: 10px;
  }

  .nes-select {
    min-width: 200px;
  }

  .empty {
    color: var(--text-secondary);
    font-size: 10px;
  }
</style>
