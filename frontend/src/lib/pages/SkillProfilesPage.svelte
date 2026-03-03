<script>
  import { onMount } from 'svelte'
  import { fullSkillProfiles, loadFullSkillProfiles, saveSkillProfile, deleteSkillProfile } from '../stores/workers.js'
  import { t } from '../stores/i18n.js'

  let showModal = false
  let editing = false
  let form = emptyForm()

  function emptyForm() {
    return {
      id: '', name: '', description: '', icon: '',
      systemPrompt: '', model: 'sonnet', permissionMode: 'acceptEdits',
      allowedTools: '', disallowedTools: '', extraCliArgs: ''
    }
  }

  onMount(() => {
    loadFullSkillProfiles()
  })

  function openCreate() {
    form = emptyForm()
    editing = false
    showModal = true
  }

  function openEdit(profile) {
    form = {
      id: profile.id,
      name: profile.name,
      description: profile.description || '',
      icon: profile.icon || '',
      systemPrompt: profile.systemPrompt || '',
      model: profile.model || 'sonnet',
      permissionMode: profile.permissionMode || 'acceptEdits',
      allowedTools: (profile.allowedTools || []).join('\n'),
      disallowedTools: (profile.disallowedTools || []).join('\n'),
      extraCliArgs: profile.extraCliArgs || ''
    }
    editing = true
    showModal = true
  }

  async function handleSave() {
    if (!form.id || !form.name) return
    const profile = {
      id: form.id,
      name: form.name,
      description: form.description,
      icon: form.icon,
      systemPrompt: form.systemPrompt,
      model: form.model,
      permissionMode: form.permissionMode,
      extraCliArgs: form.extraCliArgs,
      allowedTools: form.allowedTools.split('\n').map(s => s.trim()).filter(Boolean),
      disallowedTools: form.disallowedTools.split('\n').map(s => s.trim()).filter(Boolean)
    }
    try {
      await saveSkillProfile(profile)
      showModal = false
    } catch (e) {
      alert('Save failed: ' + (e.message || e))
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete skill profile "' + id + '"?')) return
    try {
      await deleteSkillProfile(id)
    } catch (e) {
      alert('Delete failed: ' + (e.message || e))
    }
  }

  function truncate(s, len) {
    if (!s) return ''
    return s.length > len ? s.slice(0, len) + '...' : s
  }
</script>

<div class="skills-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('skills.title')}</p>
    <div class="toolbar">
      <button class="nes-btn is-primary" on:click={openCreate}>+ {$t('skills.newProfile')}</button>
    </div>

    <div class="profile-grid">
      {#each $fullSkillProfiles as profile}
        <div class="nes-container is-dark is-rounded profile-card">
          <div class="card-header">
            <span class="profile-icon">{profile.icon || '?'}</span>
            <div class="card-title-area">
              <strong class="profile-name">{profile.name}</strong>
              {#if profile.builtIn}
                <span class="nes-badge"><span class="is-primary">{$t('skills.builtIn')}</span></span>
              {/if}
            </div>
          </div>
          <p class="profile-desc">{profile.description || ''}</p>
          <div class="card-meta">
            {#if profile.model}
              <span class="meta-tag">Model: {profile.model}</span>
            {/if}
            {#if profile.permissionMode}
              <span class="meta-tag">Mode: {profile.permissionMode}</span>
            {/if}
          </div>
          {#if profile.systemPrompt}
            <p class="system-prompt-preview">{truncate(profile.systemPrompt, 120)}</p>
          {/if}
          <div class="card-actions">
            <button class="nes-btn is-warning btn-sm" on:click={() => openEdit(profile)}>{$t('common.edit')}</button>
            {#if !profile.builtIn}
              <button class="nes-btn is-error btn-sm" on:click={() => handleDelete(profile.id)}>{$t('common.delete')}</button>
            {:else}
              <button class="nes-btn btn-sm" disabled title={$t('skills.builtInNoDelete')}>{$t('common.delete')}</button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </section>
</div>

{#if showModal}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="modal-overlay" on:click|self={() => showModal = false} role="dialog">
    <div class="nes-container is-dark is-rounded modal-box">
      <h3>{editing ? $t('skills.editProfile') : $t('skills.newProfileTitle')}</h3>
      <div class="form-grid">
        <div class="nes-field">
          <label for="sp-id">{$t('skills.id')}</label>
          <input id="sp-id" class="nes-input is-dark" bind:value={form.id} disabled={editing} placeholder="e.g. my-coder" />
        </div>
        <div class="nes-field">
          <label for="sp-name">{$t('skills.name')}</label>
          <input id="sp-name" class="nes-input is-dark" bind:value={form.name} placeholder="My Coder" />
        </div>
        <div class="nes-field">
          <label for="sp-icon">{$t('skills.icon')}</label>
          <input id="sp-icon" class="nes-input is-dark" bind:value={form.icon} placeholder="emoji" />
        </div>
        <div class="nes-field">
          <label for="sp-model">{$t('skills.model')}</label>
          <div class="nes-select is-dark">
            <select id="sp-model" bind:value={form.model}>
              <option value="">Default</option>
              <option value="sonnet">Sonnet</option>
              <option value="opus">Opus</option>
              <option value="haiku">Haiku</option>
            </select>
          </div>
        </div>
        <div class="nes-field">
          <label for="sp-perm">{$t('skills.permissionMode')}</label>
          <div class="nes-select is-dark">
            <select id="sp-perm" bind:value={form.permissionMode}>
              <option value="">Default</option>
              <option value="default">default</option>
              <option value="acceptEdits">acceptEdits</option>
              <option value="plan">plan</option>
              <option value="dontAsk">dontAsk</option>
              <option value="bypassPermissions">bypassPermissions</option>
            </select>
          </div>
        </div>
        <div class="nes-field full-width">
          <label for="sp-desc">{$t('skills.description')}</label>
          <input id="sp-desc" class="nes-input is-dark" bind:value={form.description} placeholder="Short description" />
        </div>
        <div class="nes-field full-width">
          <label for="sp-prompt">{$t('skills.systemPrompt')}</label>
          <textarea id="sp-prompt" class="nes-textarea is-dark" rows="6" bind:value={form.systemPrompt} placeholder="System prompt text..."></textarea>
        </div>
        <div class="nes-field full-width">
          <label for="sp-allowed">{$t('skills.allowedTools')}</label>
          <textarea id="sp-allowed" class="nes-textarea is-dark" rows="3" bind:value={form.allowedTools} placeholder="Bash&#10;Edit&#10;Read"></textarea>
        </div>
        <div class="nes-field full-width">
          <label for="sp-disallowed">{$t('skills.disallowedTools')}</label>
          <textarea id="sp-disallowed" class="nes-textarea is-dark" rows="3" bind:value={form.disallowedTools} placeholder="Bash(rm *)"></textarea>
        </div>
        <div class="nes-field full-width">
          <label for="sp-cli">{$t('skills.extraCliArgs')}</label>
          <input id="sp-cli" class="nes-input is-dark" bind:value={form.extraCliArgs} placeholder="--verbose" />
        </div>
      </div>
      <div class="modal-actions">
        <button class="nes-btn is-success" on:click={handleSave}>{$t('common.save')}</button>
        <button class="nes-btn" on:click={() => showModal = false}>{$t('common.cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .skills-page {
    height: 100%;
    overflow: auto;
  }

  .toolbar {
    margin-bottom: 16px;
  }

  .profile-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    gap: 16px;
  }

  .profile-card {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .profile-icon {
    font-size: 28px;
    line-height: 1;
  }

  .card-title-area {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .profile-name {
    font-size: 14px;
  }

  .profile-desc {
    font-size: 10px;
    color: var(--text-secondary, #aaa);
    margin: 0;
  }

  .card-meta {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .meta-tag {
    font-size: 9px;
    padding: 2px 6px;
    border: 2px solid var(--border-color, #555);
    color: var(--text-secondary, #aaa);
  }

  .system-prompt-preview {
    font-size: 9px;
    color: var(--text-secondary, #888);
    margin: 0;
    font-style: italic;
    line-height: 1.4;
  }

  .card-actions {
    display: flex;
    gap: 8px;
    margin-top: auto;
    padding-top: 8px;
  }

  .btn-sm {
    font-size: 9px !important;
    padding: 4px 10px !important;
  }

  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal-box {
    max-width: 640px;
    width: 90%;
    max-height: 85vh;
    overflow-y: auto;
  }

  .modal-box h3 {
    margin-top: 0;
    font-size: 14px;
  }

  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .full-width {
    grid-column: 1 / -1;
  }

  .form-grid label {
    font-size: 10px;
    margin-bottom: 4px;
    display: block;
  }

  .form-grid input,
  .form-grid textarea,
  .form-grid select {
    font-size: 10px !important;
  }

  .modal-actions {
    display: flex;
    gap: 12px;
    margin-top: 16px;
    justify-content: flex-end;
  }

  .nes-badge {
    font-size: 8px;
  }
</style>
