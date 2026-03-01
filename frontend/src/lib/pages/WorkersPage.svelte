<script>
  import { onMount } from 'svelte'
  import { workers, loadWorkers, createWorker } from '../stores/workers.js'
  import WorkerCard from '../components/WorkerCard.svelte'
  import WorkerLogPanel from '../components/WorkerLogPanel.svelte'
  import { addError } from '../stores/errors.js'

  let showHire = false
  let newName = ''
  let newAvatar = 'robot'
  let logWorker = null

  const avatarOptions = [
    { id: 'robot', label: 'Robot' },
    { id: 'kirby', label: 'Kirby' },
    { id: 'mario', label: 'Mario' },
    { id: 'ash', label: 'Ash' },
    { id: 'bulbasaur', label: 'Bulbasaur' },
    { id: 'charmander', label: 'Charmander' },
    { id: 'squirtle', label: 'Squirtle' },
    { id: 'pokeball', label: 'Pokeball' },
  ]

  onMount(async () => {
    try {
      await loadWorkers()
    } catch (e) {
      addError('Failed to load workers: ' + e.message)
    }
  })

  async function handleHire() {
    if (!newName) return
    try {
      await createWorker(newName, newAvatar)
      newName = ''
      newAvatar = 'robot'
      showHire = false
    } catch (e) {
      addError('Failed to hire worker: ' + e.message)
    }
  }
</script>

<div class="workers-page">
  <section class="nes-container with-title is-dark">
    <p class="title">Workers</p>
    <div class="toolbar">
      <button class="nes-btn is-success" on:click={() => showHire = true}>+ Hire Worker</button>
    </div>

    <div class="workers-grid">
      {#each $workers as w}
        <WorkerCard worker={w} onClick={(worker) => logWorker = worker} />
      {/each}
      {#if $workers.length === 0}
        <p class="empty-msg">No workers hired yet. Hire AI employees to start working!</p>
      {/if}
    </div>
  </section>

  {#if logWorker}
    <WorkerLogPanel
      workerId={logWorker.id}
      workerName={logWorker.name}
      onClose={() => logWorker = null}
    />
  {/if}

  {#if showHire}
    <div class="dialog-overlay" on:click={() => showHire = false} on:keydown={(e) => e.key === 'Escape' && (showHire = false)} role="presentation">
      <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
        <p class="title">Hire Worker</p>
        <form on:submit|preventDefault={handleHire}>
          <div class="nes-field">
            <label for="w-name">Name</label>
            <input type="text" id="w-name" class="nes-input is-dark" bind:value={newName} placeholder="e.g. Alice" />
          </div>
          <div class="nes-field">
            <label>Avatar</label>
            <div class="avatar-grid">
              {#each avatarOptions as opt}
                <label class="avatar-option" class:selected={newAvatar === opt.id}>
                  <input type="radio" class="nes-radio is-dark" name="avatar" value={opt.id} bind:group={newAvatar} />
                  <span>{opt.label}</span>
                </label>
              {/each}
            </div>
          </div>
          <div class="dialog-actions">
            <button type="submit" class="nes-btn is-success">Hire</button>
            <button type="button" class="nes-btn" on:click={() => showHire = false}>Cancel</button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>

<style>
  .workers-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
  }

  .toolbar {
    margin-bottom: 12px;
  }

  .toolbar button {
    font-size: 10px;
  }

  .workers-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
  }

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
    width: 400px;
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

  .nes-field input[type="text"] {
    font-size: 10px;
    width: 100%;
  }

  .avatar-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .avatar-option {
    font-size: 9px;
    padding: 4px 8px;
    border: 2px solid transparent;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .avatar-option.selected {
    border-color: var(--accent-green);
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
