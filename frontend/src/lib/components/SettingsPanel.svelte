<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'

  export let config = {}

  $: polling = config?.polling || {}
  $: decision = config?.decision || {}
  $: context = config?.context || {}

  let chatBackend = ''
  let availableChatBackends = []

  onMount(async () => {
    if (window.go?.gui?.CompanyApp) {
      try {
        chatBackend = await window.go.gui.CompanyApp.GetChatBackend()
        availableChatBackends = await window.go.gui.CompanyApp.GetAvailableChatBackends()
      } catch {
        // ignore
      }
    }
  })

  async function onChatBackendChange(e) {
    const name = e.target.value
    if (window.go?.gui?.CompanyApp) {
      try {
        await window.go.gui.CompanyApp.SetChatBackend(name)
        chatBackend = name
      } catch (err) {
        console.error('Failed to set chat backend:', err)
      }
    }
  }
</script>

<div class="settings-panel">
  <div class="setting-group nes-container with-title is-dark">
    <p class="title">{$t('settings.chatBackend')}</p>
    <div class="field">
      <label>{$t('settings.chatBackend')}:</label>
      {#if availableChatBackends.length > 0}
        <select class="nes-select is-dark chat-select" value={chatBackend} on:change={onChatBackendChange}>
          {#each availableChatBackends as backend}
            <option value={backend}>{backend}</option>
          {/each}
        </select>
      {:else}
        <span class="value">{chatBackend || 'N/A'}</span>
      {/if}
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Polling</p>
    <div class="field">
      <label>Interval (ms):</label>
      <span class="value">{polling.interval_ms || 500}</span>
    </div>
    <div class="field">
      <label>Context Lines:</label>
      <span class="value">{polling.context_lines || 100}</span>
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Decision</p>
    <div class="field">
      <label>Confidence Threshold:</label>
      <span class="value">{decision.confidence_threshold || 0.7}</span>
    </div>
    <div class="field">
      <label>Timeout (s):</label>
      <span class="value">{decision.timeout_seconds || 30}</span>
    </div>
  </div>

  <div class="setting-group nes-container with-title is-dark">
    <p class="title">Context</p>
    <div class="field">
      <label>Enabled:</label>
      <span class="value">{context.enabled ? 'Yes' : 'No'}</span>
    </div>
    <div class="field">
      <label>Max Decisions:</label>
      <span class="value">{context.max_decisions || 20}</span>
    </div>
    <div class="field">
      <label>Token Budget:</label>
      <span class="value">{context.token_budget || 2000}</span>
    </div>
  </div>
</div>

<style>
  .settings-panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .setting-group {
    font-size: 10px;
  }

  .field {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 0;
    border-bottom: 1px solid var(--border-color);
  }

  label {
    color: var(--text-secondary);
  }

  .value {
    color: var(--accent-green);
  }

  .chat-select {
    font-size: 10px;
    max-width: 160px;
  }
</style>
