<script>
  import { onMount } from 'svelte'
  import { activeDiscussions } from '../stores/discussions.js'
  import GroupDiscussion from '../components/GroupDiscussion.svelte'

  let groups = []

  onMount(async () => {
    if (window.go?.gui?.App) {
      groups = (await window.go.gui.App.GetGroups()) || []
    }
  })
</script>

<div class="groups-page">
  <section class="nes-container with-title is-dark">
    <p class="title">Groups</p>
    {#if groups.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Leader</th>
            <th>Roles</th>
            <th>Threshold</th>
          </tr>
        </thead>
        <tbody>
          {#each groups as group}
            <tr>
              <td>{group.id}</td>
              <td>{group.name}</td>
              <td>{group.leaderId}</td>
              <td>{(group.roleIds || []).join(', ')}</td>
              <td>{group.divergenceThreshold}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <p class="empty">No groups configured</p>
    {/if}
  </section>

  <section class="nes-container with-title is-dark discussions-section">
    <p class="title">Active Discussions</p>
    {#if $activeDiscussions.length > 0}
      {#each $activeDiscussions as disc}
        <div class="nes-container is-rounded discussion-container">
          <div class="disc-header">
            <span class="disc-id">{disc.id}</span>
            <span class="nes-badge">
              <span class="is-warning">{disc.latestPhase}</span>
            </span>
          </div>
          <GroupDiscussion discussion={disc} />
        </div>
      {/each}
    {:else}
      <p class="empty">No active discussions</p>
    {/if}
  </section>
</div>

<style>
  .groups-page {
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

  .discussions-section {
    flex: 1;
  }

  .discussion-container {
    margin-bottom: 16px;
  }

  .disc-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
    font-size: 10px;
  }

  .disc-id {
    color: var(--accent-blue);
    font-size: 9px;
  }

  .empty {
    color: var(--text-secondary);
    font-size: 10px;
    text-align: center;
    padding: 20px;
  }
</style>
