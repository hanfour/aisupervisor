<script>
  import { chatCreateProject, createProject, loadProjects } from '../stores/projects.js'
  import { addError } from '../stores/errors.js'

  export let visible = false
  export let onClose = () => {}

  let messages = []
  let userInput = ''
  let loading = false
  let projectReady = null

  function reset() {
    messages = []
    userInput = ''
    loading = false
    projectReady = null
  }

  function handleClose() {
    reset()
    onClose()
  }

  async function sendMessage() {
    if (!userInput.trim() || loading) return

    const text = userInput.trim()
    userInput = ''
    messages = [...messages, { role: 'user', content: text }]
    loading = true

    try {
      const apiMessages = messages.map(m => ({ role: m.role, content: m.content }))
      const resp = await chatCreateProject(apiMessages)

      if (!resp) {
        addError('No response from AI')
        loading = false
        return
      }

      if (resp.status === 'ready') {
        projectReady = resp
        const summary = `I have all the information needed to create your project:\n- Name: ${resp.name}\n- Description: ${resp.description}\n- Repo: ${resp.repoPath}\n- Branch: ${resp.baseBranch}${resp.goals && resp.goals.length > 0 ? '\n- Goals: ' + resp.goals.join(', ') : ''}`
        messages = [...messages, { role: 'assistant', content: summary }]
      } else {
        const questionText = resp.questions && resp.questions.length > 0
          ? resp.questions.join('\n')
          : 'Could you provide more details?'
        messages = [...messages, { role: 'assistant', content: questionText }]
      }
    } catch (e) {
      addError('AI chat error: ' + e.message)
      messages = [...messages, { role: 'assistant', content: 'Sorry, an error occurred. Please try again.' }]
    }

    loading = false
  }

  async function confirmCreate() {
    if (!projectReady) return
    loading = true
    try {
      await createProject(
        projectReady.name,
        projectReady.description,
        projectReady.repoPath,
        projectReady.baseBranch,
        projectReady.goals || []
      )
      await loadProjects()
      handleClose()
    } catch (e) {
      addError('Failed to create project: ' + e.message)
    }
    loading = false
  }

  function handleKeydown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
    if (e.key === 'Escape') {
      handleClose()
    }
  }
</script>

{#if visible}
  <div class="dialog-overlay" on:click={handleClose} on:keydown={(e) => e.key === 'Escape' && handleClose()} role="presentation">
    <div class="chat-dialog nes-container is-dark is-rounded" on:click|stopPropagation role="presentation">
      <p class="title">AI Project Creator</p>

      <div class="chat-messages">
        {#if messages.length === 0}
          <div class="hint">
            Describe the project you want to create. The AI will ask follow-up questions if needed.
          </div>
        {/if}

        {#each messages as msg}
          <div class="message {msg.role}">
            <div class="bubble {msg.role}">
              <pre class="msg-text">{msg.content}</pre>
            </div>
          </div>
        {/each}

        {#if loading}
          <div class="message assistant">
            <div class="bubble assistant">
              <span class="typing">Thinking...</span>
            </div>
          </div>
        {/if}
      </div>

      {#if projectReady}
        <div class="project-preview nes-container is-rounded">
          <div class="preview-row"><span class="label">Name:</span> {projectReady.name}</div>
          <div class="preview-row"><span class="label">Desc:</span> {projectReady.description}</div>
          <div class="preview-row"><span class="label">Repo:</span> {projectReady.repoPath}</div>
          <div class="preview-row"><span class="label">Branch:</span> {projectReady.baseBranch}</div>
          {#if projectReady.goals && projectReady.goals.length > 0}
            <div class="preview-row"><span class="label">Goals:</span> {projectReady.goals.join(', ')}</div>
          {/if}
        </div>
        <div class="dialog-actions">
          <button class="nes-btn is-success" on:click={confirmCreate} disabled={loading}>Create Project</button>
          <button class="nes-btn" on:click={handleClose}>Cancel</button>
        </div>
      {:else}
        <div class="chat-input-row">
          <textarea
            class="nes-textarea is-dark"
            bind:value={userInput}
            on:keydown={handleKeydown}
            placeholder="Describe your project..."
            rows="2"
            disabled={loading}
          ></textarea>
          <button class="nes-btn is-primary" on:click={sendMessage} disabled={loading || !userInput.trim()}>
            Send
          </button>
        </div>
      {/if}
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

  .chat-dialog {
    width: 540px;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    padding: 20px !important;
  }

  .title {
    color: var(--accent-yellow);
    margin-bottom: 12px;
    font-size: 12px;
  }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    max-height: 350px;
    min-height: 120px;
    margin-bottom: 12px;
    padding: 4px;
  }

  .hint {
    font-size: 9px;
    color: var(--text-secondary);
    text-align: center;
    padding: 20px 0;
  }

  .message {
    display: flex;
    margin-bottom: 8px;
  }

  .message.user {
    justify-content: flex-end;
  }

  .message.assistant {
    justify-content: flex-start;
  }

  .bubble {
    max-width: 85%;
    padding: 8px 12px;
    font-size: 9px;
    border: 2px solid;
  }

  .bubble.user {
    border-color: var(--accent-blue);
    background: rgba(41, 98, 255, 0.1);
  }

  .bubble.assistant {
    border-color: var(--accent-green);
    background: rgba(0, 230, 118, 0.1);
  }

  .msg-text {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    font-family: inherit;
    font-size: inherit;
  }

  .typing {
    animation: blink 1s step-end infinite;
  }

  @keyframes blink {
    50% { opacity: 0; }
  }

  .project-preview {
    font-size: 9px;
    padding: 8px !important;
    margin-bottom: 12px;
    border-color: var(--accent-green) !important;
  }

  .preview-row {
    margin: 4px 0;
  }

  .preview-row .label {
    color: var(--text-secondary);
  }

  .chat-input-row {
    display: flex;
    gap: 8px;
    align-items: flex-end;
  }

  .chat-input-row textarea {
    flex: 1;
    font-size: 10px;
    resize: none;
  }

  .chat-input-row button {
    font-size: 10px;
    white-space: nowrap;
  }

  .dialog-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }

  .dialog-actions button {
    font-size: 10px;
  }
</style>
