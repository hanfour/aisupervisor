<script>
  import { roles } from '../stores/roles.js'

  export let sessionId = ''
  export let assignedRoleIds = []
  export let onUpdate = () => {}

  function toggle(roleId) {
    if (assignedRoleIds.includes(roleId)) {
      assignedRoleIds = assignedRoleIds.filter(id => id !== roleId)
    } else {
      assignedRoleIds = [...assignedRoleIds, roleId]
    }
    onUpdate(sessionId, assignedRoleIds)
  }

  function isAssigned(roleId) {
    return assignedRoleIds.includes(roleId)
  }

  // NES.css avatar: use backend avatar if set, otherwise infer from role name
  function avatarIcon(role) {
    if (role.avatar) return role.avatar
    const name = (role.name || '').toLowerCase()
    if (name.includes('gatekeeper')) return 'nes-icon is-medium star'
    if (name.includes('manager') || name.includes('rd')) return 'nes-mario'
    if (name.includes('pm') || name.includes('product')) return 'nes-ash'
    if (name.includes('security')) return 'nes-kirby'
    return 'nes-icon is-medium heart'
  }
</script>

<div class="role-assignment">
  <h3>Roles for: {sessionId}</h3>
  <div class="role-list">
    {#each $roles as role}
      <label class="role-item">
        <input
          type="checkbox"
          class="nes-checkbox is-dark"
          checked={isAssigned(role.id)}
          on:change={() => toggle(role.id)}
        />
        <span class="role-info">
          <i class={avatarIcon(role)}></i>
          <span class="role-name">{role.name}</span>
          <span class="role-mode nes-badge">
            <span class="is-primary">{role.mode}</span>
          </span>
          <span class="role-priority">P{role.priority}</span>
        </span>
      </label>
    {/each}
  </div>
</div>

<style>
  .role-assignment h3 {
    font-size: 11px;
    color: var(--accent-blue);
    margin-bottom: 12px;
  }

  .role-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .role-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px;
    border: 2px solid var(--border-color);
    cursor: pointer;
  }

  .role-item:hover {
    border-color: var(--accent-blue);
  }

  .role-info {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 10px;
  }

  .role-name {
    color: var(--text-primary);
  }

  .role-priority {
    color: var(--text-secondary);
    font-size: 9px;
  }
</style>
