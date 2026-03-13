<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'
  import { addError } from '../stores/errors.js'

  export let onNavigate = () => {}

  let objectives = []
  let showCreate = false
  let newTitle = ''
  let newDescription = ''
  let newBudgetLimit = 0
  let decomposing = {}

  onMount(loadObjectives)

  async function loadObjectives() {
    try {
      objectives = await window.go.gui.CompanyApp.ListObjectives() || []
    } catch (e) {
      addError('Failed to load objectives: ' + e.message)
    }
  }

  async function handleCreate() {
    if (!newTitle) return
    try {
      await window.go.gui.CompanyApp.CreateObjective(newTitle, newDescription, newBudgetLimit)
      newTitle = ''
      newDescription = ''
      newBudgetLimit = 0
      showCreate = false
      await loadObjectives()
    } catch (e) {
      addError('Failed to create objective: ' + e.message)
    }
  }

  async function handleDecompose(objId) {
    decomposing[objId] = true
    try {
      await window.go.gui.CompanyApp.DecomposeObjective(objId)
      await loadObjectives()
    } catch (e) {
      addError('Failed to decompose: ' + e.message)
    } finally {
      decomposing[objId] = false
    }
  }

  async function handleDelete(objId) {
    try {
      await window.go.gui.CompanyApp.DeleteObjective(objId)
      await loadObjectives()
    } catch (e) {
      addError('Failed to delete: ' + e.message)
    }
  }

  async function handleUpdateKR(objId, krId, value) {
    try {
      await window.go.gui.CompanyApp.UpdateKeyResult(objId, krId, parseFloat(value))
      await loadObjectives()
    } catch (e) {
      addError('Failed to update KR: ' + e.message)
    }
  }

  function krProgress(kr) {
    if (!kr.target || kr.target <= 0) return 0
    const p = (kr.current / kr.target) * 100
    return Math.min(p, 100)
  }
</script>

<div class="objectives-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('objectives.title')}</p>
    <div class="toolbar">
      <button class="nes-btn is-success" on:click={() => showCreate = true}>{$t('objectives.create')}</button>
    </div>

    {#if objectives.length === 0}
      <p class="empty-msg">{$t('objectives.empty')}</p>
    {/if}

    <div class="objectives-grid">
      {#each objectives as obj}
        <div class="nes-container is-rounded objective-card" class:is-dark={obj.status === 'active'} class:is-disabled={obj.status !== 'active'}>
          <div class="obj-header">
            <h3 class="obj-title">{obj.title}</h3>
            <span class="obj-status nes-badge"><span class:is-success={obj.status === 'active'} class:is-warning={obj.status === 'completed'}>{obj.status}</span></span>
          </div>
          {#if obj.description}
            <p class="obj-desc">{obj.description}</p>
          {/if}

          {#if obj.keyResults && obj.keyResults.length > 0}
            <div class="kr-list">
              <h4 class="kr-header">{$t('objectives.keyResults')}</h4>
              {#each obj.keyResults as kr}
                <div class="kr-item">
                  <span class="kr-title">{kr.title}</span>
                  <div class="kr-bar-wrap">
                    <div class="kr-bar" style="width: {krProgress(kr)}%"></div>
                  </div>
                  <span class="kr-value">{kr.current}/{kr.target} {kr.unit}</span>
                </div>
              {/each}
            </div>
          {/if}

          {#if obj.projectIds && obj.projectIds.length > 0}
            <div class="linked-projects">
              <span class="link-label">{$t('objectives.linkedProjects')}:</span>
              {#each obj.projectIds as pid}
                <button class="nes-btn is-primary link-btn" on:click={() => onNavigate('board', pid)}>{pid.slice(0, 8)}</button>
              {/each}
            </div>
          {/if}

          <div class="obj-actions">
            <button class="nes-btn is-primary" on:click={() => handleDecompose(obj.id)} disabled={decomposing[obj.id]}>
              {decomposing[obj.id] ? $t('objectives.decomposing') : $t('objectives.decompose')}
            </button>
            <button class="nes-btn is-error" on:click={() => handleDelete(obj.id)}>{$t('common.delete')}</button>
          </div>
        </div>
      {/each}
    </div>
  </section>

  {#if showCreate}
    <div class="dialog-overlay" on:click={() => showCreate = false} on:keydown={(e) => e.key === 'Escape' && (showCreate = false)} role="presentation">
      <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
        <p class="title">{$t('objectives.create')}</p>
        <form on:submit|preventDefault={handleCreate}>
          <div class="nes-field">
            <label for="obj-title">{$t('objectives.titleLabel')}</label>
            <input type="text" id="obj-title" class="nes-input is-dark" bind:value={newTitle} />
          </div>
          <div class="nes-field">
            <label for="obj-desc">{$t('objectives.descLabel')}</label>
            <textarea id="obj-desc" class="nes-textarea is-dark" bind:value={newDescription}></textarea>
          </div>
          <div class="nes-field">
            <label for="obj-budget">{$t('objectives.budgetLabel')}</label>
            <input type="number" id="obj-budget" class="nes-input is-dark" bind:value={newBudgetLimit} />
          </div>
          <div class="dialog-actions">
            <button type="submit" class="nes-btn is-success">{$t('objectives.create')}</button>
            <button type="button" class="nes-btn" on:click={() => showCreate = false}>{$t('common.cancel')}</button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>

<style>
  .objectives-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
    overflow-y: auto;
  }

  .toolbar {
    margin-bottom: 12px;
  }

  .toolbar button {
    font-size: 10px;
  }

  .objectives-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .objective-card {
    padding: 12px !important;
  }

  .obj-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .obj-title {
    font-size: 12px;
    margin: 0;
    color: var(--accent-green);
  }

  .obj-status {
    font-size: 8px;
  }

  .obj-desc {
    font-size: 9px;
    color: var(--text-secondary);
    margin: 0 0 8px;
  }

  .kr-header {
    font-size: 10px;
    margin: 0 0 6px;
    color: var(--accent-blue);
  }

  .kr-list {
    margin-bottom: 8px;
  }

  .kr-item {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 4px;
  }

  .kr-title {
    font-size: 9px;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kr-bar-wrap {
    width: 80px;
    height: 8px;
    background: rgba(255,255,255,0.1);
    border: 1px solid var(--border-color);
  }

  .kr-bar {
    height: 100%;
    background: var(--accent-green);
    transition: width 0.3s;
  }

  .kr-value {
    font-size: 8px;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .linked-projects {
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
  }

  .link-label {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .link-btn {
    font-size: 7px;
    padding: 1px 4px;
  }

  .obj-actions {
    display: flex;
    gap: 8px;
  }

  .obj-actions button {
    font-size: 8px;
    padding: 2px 8px;
  }

  .dialog-overlay {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0,0,0,0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .nes-dialog {
    width: 450px;
    max-height: 85vh;
    overflow-y: auto;
    padding: 24px !important;
  }

  .nes-field {
    margin-bottom: 12px;
  }

  .nes-field label {
    font-size: 10px;
    margin-bottom: 4px;
    display: block;
  }

  .nes-field input, .nes-field textarea {
    font-size: 10px;
    width: 100%;
  }

  .nes-field textarea {
    min-height: 60px;
  }

  .dialog-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .dialog-actions button {
    font-size: 10px;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
  }
</style>
