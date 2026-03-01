<script>
  import { createTask } from '../stores/tasks.js'
  import { addError } from '../stores/errors.js'

  export let visible = false
  export let projectId = ''
  export let existingTasks = []
  export let onClose = () => {}

  let title = ''
  let description = ''
  let prompt = ''
  let priority = 2
  let milestone = ''
  let selectedDeps = []

  async function handleSubmit() {
    if (!title || !prompt) return
    try {
      await createTask(projectId, title, description, prompt, selectedDeps, priority, milestone)
      title = ''
      description = ''
      prompt = ''
      priority = 2
      milestone = ''
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
      <p class="title">New Task</p>
      <form on:submit|preventDefault={handleSubmit}>
        <div class="nes-field">
          <label for="task-title">Title</label>
          <input type="text" id="task-title" class="nes-input is-dark" bind:value={title} />
        </div>
        <div class="nes-field">
          <label for="task-desc">Description</label>
          <textarea id="task-desc" class="nes-textarea is-dark" bind:value={description} rows="2"></textarea>
        </div>
        <div class="nes-field">
          <label for="task-prompt">Prompt (for Claude Code)</label>
          <textarea id="task-prompt" class="nes-textarea is-dark" bind:value={prompt} rows="4"></textarea>
        </div>
        <div class="field-row">
          <div class="nes-field">
            <label for="task-priority">Priority</label>
            <input type="number" id="task-priority" class="nes-input is-dark" bind:value={priority} min="1" max="9" />
          </div>
          <div class="nes-field">
            <label for="task-milestone">Milestone</label>
            <input type="text" id="task-milestone" class="nes-input is-dark" bind:value={milestone} />
          </div>
        </div>
        {#if existingTasks.length > 0}
          <div class="nes-field">
            <label>Dependencies</label>
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
          <button type="submit" class="nes-btn is-primary">Create</button>
          <button type="button" class="nes-btn" on:click={onClose}>Cancel</button>
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
