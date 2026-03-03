<script>
  import { onMount } from 'svelte'
  import SettingsPanel from '../components/SettingsPanel.svelte'
  import { t, setLanguage as setI18nLanguage } from '../stores/i18n.js'

  let config = {}
  let language = 'zh-TW'

  onMount(async () => {
    if (window.go?.gui?.App) {
      config = (await window.go.gui.App.GetConfig()) || {}
    }
    if (window.go?.gui?.CompanyApp) {
      language = (await window.go.gui.CompanyApp.GetLanguage()) || 'zh-TW'
    }
  })

  async function handleLanguageChange() {
    await setI18nLanguage(language)
  }
</script>

<div class="settings-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.language')}</p>
    <div class="lang-select">
      <label>
        <input type="radio" class="nes-radio is-dark" name="lang" value="en"
          bind:group={language} on:change={handleLanguageChange} />
        <span>English</span>
      </label>
      <label>
        <input type="radio" class="nes-radio is-dark" name="lang" value="zh-TW"
          bind:group={language} on:change={handleLanguageChange} />
        <span>繁體中文</span>
      </label>
    </div>
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.configuration')}</p>
    <SettingsPanel {config} />
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.backends')}</p>
    {#if config.backends && config.backends.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>{$t('settings.nameCol')}</th>
            <th>{$t('settings.typeCol')}</th>
            <th>{$t('settings.modelCol')}</th>
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
      <p class="empty">{$t('settings.noBackends')}</p>
    {/if}
  </section>

  <section class="nes-container with-title is-dark">
    <p class="title">{$t('settings.autoApprove')}</p>
    {#if config.auto_approve_rules && config.auto_approve_rules.length > 0}
      <table class="nes-table is-bordered is-dark">
        <thead>
          <tr>
            <th>{$t('settings.labelCol')}</th>
            <th>{$t('settings.patternCol')}</th>
            <th>{$t('settings.responseCol')}</th>
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
      <p class="empty">{$t('settings.noAutoApprove')}</p>
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

  .lang-select {
    display: flex;
    gap: 24px;
  }

  .lang-select label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 12px;
  }
</style>
