<script>
  import { createTask } from '../stores/tasks.js'
  import { addError } from '../stores/errors.js'
  import { t } from '../stores/i18n.js'

  export let visible = false
  export let projectId = ''
  export let existingTasks = []
  export let onClose = () => {}

  let title = ''
  let description = ''
  let prompt = ''
  let priority = 2
  let milestone = ''
  let taskType = 'code'
  let selectedDeps = []

  async function handleSubmit() {
    if (!title || !prompt) return
    try {
      await createTask(projectId, title, description, prompt, selectedDeps, priority, milestone, taskType)
      title = ''
      description = ''
      prompt = ''
      priority = 2
      milestone = ''
      taskType = 'code'
      selectedDeps = []
      onClose()
    } catch (e) {
      addError('Failed to create task: ' + e.message)
    }
  }

  function toggleDep(taskId) {
    if (selectedDeps.includes(taskId)) {
      selectedDeps = selectedDeps.filter(d => d !== taskId)
    } else {
      selectedDeps = [...selectedDeps, taskId]
    }
  }
</script>

{#if visible}
  <div class="dialog-overlay" on:click={onClose} on:keydown={(e) => e.key === 'Escape' && onClose()} role="presentation">
    <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
      <p class="title">{$t('taskForm.title')}</p>
      <form on:submit|preventDefault={handleSubmit}>
        <div class="nes-field">
          <label for="task-title">{$t('taskForm.titleLabel')}</label>
          <input type="text" id="task-title" class="nes-input is-dark" bind:value={title} />
        </div>
        <div class="nes-field">
          <label for="task-desc">{$t('taskForm.description')}</label>
          <textarea id="task-desc" class="nes-textarea is-dark" bind:value={description} rows="2"></textarea>
        </div>
        <div class="nes-field">
          <label for="task-prompt">{$t('taskForm.prompt')}</label>
          <textarea id="task-prompt" class="nes-textarea is-dark" bind:value={prompt} rows="4"></textarea>
        </div>
        <div class="field-row">
          <div class="nes-field">
            <label for="task-type">{$t('taskForm.type')}</label>
            <div class="type-selector">
              <label class="type-option" class:selected={taskType === 'code'}>
                <input type="radio" class="nes-radio is-dark" name="taskType" value="code" bind:group={taskType} />
                <span>{$t('taskForm.typeCode')}</span>
              </label>
              <label class="type-option" class:selected={taskType === 'research'}>
                <input type="radio" class="nes-radio is-dark" name="taskType" value="research" bind:group={taskType} />
                <span>{$t('taskForm.typeResearch')}</span>
              </label>
            </div>
          </div>
          <div class="nes-field">
            <label for="task-priority">{$t('taskForm.priority')}</label>
            <input type="number" id="task-priority" class="nes-input is-dark" bind:value={priority} min="1" max="9" />
          </div>
          <div class="nes-field">
            <label for="task-milestone">{$t('taskForm.milestone')}</label>
            <input type="text" id="task-milestone" class="nes-input is-dark" bind:value={milestone} />
          </div>
        </div>
        {#if existingTasks.length > 0}
          <div class="nes-field">
            <label>{$t('taskForm.dependencies')}</label>
            <div class="deps-list">
              {#each existingTasks as t}
                <label class="dep-item">
                  <input
                    type="checkbox"
                    class="nes-checkbox is-dark"
                    checked={selectedDeps.includes(t.id)}
                    on:change={() => toggleDep(t.id)}
                  />
                  <span>{t.title}</span>
                </label>
              {/each}
            </div>
          </div>
        {/if}
        <div class="dialog-actions">
          <button type="submit" class="nes-btn is-primary">{$t('taskForm.create')}</button>
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
    width: 520px;
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

  .field-row {
    display: flex;
    gap: 12px;
  }

  .field-row .nes-field {
    flex: 1;
  }

  .type-selector {
    display: flex;
    gap: 12px;
  }

  .type-option {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 9px;
    cursor: pointer;
  }

  .type-option.selected {
    color: var(--accent-green);
  }

  .deps-list {
    max-height: 120px;
    overflow-y: auto;
  }

  .dep-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 9px;
    margin: 4px 0;
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
