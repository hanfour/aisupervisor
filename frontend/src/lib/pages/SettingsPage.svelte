<script>
  import { onMount } from 'svelte'
  import SettingsPanel from '../components/SettingsPanel.svelte'

  let config = {}

  onMount(async () => {
    if (window.go?.gui?.App) {
      config = (await window.go.gui.App.GetConfig()) || {}
    }
  })
</script>

<div class="settings-page">
  <section class="nes-container with-title is-dark">
    <p class="title">Configuration</p>
    <SettingsPanel {config} />
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">Backends</p>
    {#if config.backends && config.backends.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Model</th>
          </tr>
        </thead>
        <tbody>
          {#each config.backends as backend}
            <tr>
              <td>{backend.name}</td>
              <td>{backend.type}</td>
              <td>{backend.model || '-'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <p class="empty">No backends configured</p>
    {/if}
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">Auto-Approve Rules</p>
    {#if config.auto_approve_rules && config.auto_approve_rules.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>Label</th>
            <th>Pattern</th>
            <th>Response</th>
          </tr>
        </thead>
        <tbody>
          {#each config.auto_approve_rules as rule}
            <tr>
              <td>{rule.label}</td>
              <td>{rule.pattern_contains}</td>
              <td>{rule.response}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <p class="empty">No auto-approve rules</p>
    {/if}
  </section>
</div>

<style>
  .settings-page {
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

  .empty {
    color: var(--text-secondary);
    font-size: 10px;
  }
</style>
