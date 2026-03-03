<script>
  import { createProject } from '../stores/projects.js'
  import { addError } from '../stores/errors.js'
  import { t } from '../stores/i18n.js'

  export let visible = false
  export let onClose = () => {}

  let name = ''
  let description = ''
  let repoPath = ''
  let baseBranch = 'main'
  let goalsText = ''

  async function handleSubmit() {
    if (!name || !repoPath) return
    try {
      const goals = goalsText.split('\n').map(g => g.trim()).filter(Boolean)
      await createProject(name, description, repoPath, baseBranch, goals)
      name = ''
      description = ''
      repoPath = ''
      baseBranch = 'main'
      goalsText = ''
      onClose()
    } catch (e) {
      addError('Failed to create project: ' + e.message)
    }
  }
</script>

{#if visible}
  <div class="dialog-overlay" on:click={onClose} on:keydown={(e) => e.key === 'Escape' && onClose()} role="presentation">
    <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
      <p class="title">{$t('projectForm.title')}</p>
      <form on:submit|preventDefault={handleSubmit}>
        <div class="nes-field">
          <label for="proj-name">{$t('projectForm.name')}</label>
          <input type="text" id="proj-name" class="nes-input is-dark" bind:value={name} />
        </div>
        <div class="nes-field">
          <label for="proj-desc">{$t('projectForm.description')}</label>
          <textarea id="proj-desc" class="nes-textarea is-dark" bind:value={description} rows="2"></textarea>
        </div>
        <div class="nes-field">
          <label for="proj-repo">{$t('projectForm.repoPath')}</label>
          <input type="text" id="proj-repo" class="nes-input is-dark" bind:value={repoPath} placeholder="/path/to/repo" />
        </div>
        <div class="nes-field">
          <label for="proj-branch">{$t('projectForm.baseBranch')}</label>
          <input type="text" id="proj-branch" class="nes-input is-dark" bind:value={baseBranch} />
        </div>
        <div class="nes-field">
          <label for="proj-goals">{$t('projectForm.goalsLabel')}</label>
          <textarea id="proj-goals" class="nes-textarea is-dark" bind:value={goalsText} rows="3"></textarea>
        </div>
        <div class="dialog-actions">
          <button type="submit" class="nes-btn is-primary">{$t('projectForm.create')}</button>
          <button type="button" class="nes-btn" on:click={onClose}>{$t('common.cancel')}</button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .dialog-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0,0,0,0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .nes-dialog {
    width: 480px;
    max-height: 80vh;
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

  .dialog-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .dialog-actions button {
    font-size: 10px;
  }
</style>
