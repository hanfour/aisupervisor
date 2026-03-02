<script>
  import { onMount } from 'svelte'
  import { workers, loadWorkers, createWorkerWithTier, promoteWorker, hierarchy, loadHierarchy, skillProfiles, loadSkillProfiles } from '../stores/workers.js'
  import WorkerCard from '../components/WorkerCard.svelte'
  import WorkerDetailDrawer from '../components/WorkerDetailDrawer.svelte'
  import { addError } from '../stores/errors.js'

  let showHire = false
  let newName = ''
  let newAvatar = 'robot'
  let newTier = 'engineer'
  let newParentID = ''
  let newBackendID = ''
  let newCliTool = 'claude'
  let newSkillProfile = ''
  let selectedWorkerId = null

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

  const tierOptions = [
    { id: 'consultant', label: 'Consultant' },
    { id: 'manager', label: 'Manager' },
    { id: 'engineer', label: 'Engineer' },
  ]

  const cliToolOptions = [
    { id: 'claude', label: 'Claude' },
    { id: 'codex', label: 'Codex' },
    { id: 'gemini', label: 'Gemini' },
  ]

  // Available managers for parent selection
  $: managers = [...($hierarchy.consultant || []), ...($hierarchy.manager || [])]

  onMount(async () => {
    try {
      await loadWorkers()
      await loadHierarchy()
      await loadSkillProfiles()
    } catch (e) {
      addError('Failed to load workers: ' + e.message)
    }
  })

  async function handleHire() {
    if (!newName) return
    try {
      await createWorkerWithTier(newName, newAvatar, newTier, newParentID, newBackendID, newCliTool, newSkillProfile)
      newName = ''
      newAvatar = 'robot'
      newTier = 'engineer'
      newParentID = ''
      newBackendID = ''
      newCliTool = 'claude'
      newSkillProfile = ''
      showHire = false
    } catch (e) {
      addError('Failed to hire worker: ' + e.message)
    }
  }

  async function handlePromote(worker) {
    const tiers = ['engineer', 'manager', 'consultant']
    const currentIdx = tiers.indexOf(worker.tier)
    if (currentIdx < 0 || currentIdx >= tiers.length - 1) return
    const nextTier = tiers[currentIdx + 1]
    try {
      await promoteWorker(worker.id, nextTier)
    } catch (e) {
      addError('Failed to promote worker: ' + e.message)
    }
  }

  function tierLabel(tier) {
    return tier.charAt(0).toUpperCase() + tier.slice(1) + 's'
  }

  const tierOrder = ['consultant', 'manager', 'engineer']
</script>

<div class="workers-page">
  <section class="nes-container with-title is-dark">
    <p class="title">Workers</p>
    <div class="toolbar">
      <button class="nes-btn is-success" on:click={() => showHire = true}>+ Hire Worker</button>
    </div>

    <div class="hierarchy-grid">
      {#each tierOrder as tier}
        {@const tierWorkers = $hierarchy[tier] || []}
        <div class="tier-column">
          <h3 class="tier-header">{tierLabel(tier)} ({tierWorkers.length})</h3>
          <div class="tier-workers">
            {#each tierWorkers as w}
              <div class="worker-wrap">
                <WorkerCard worker={w} onClick={(worker) => selectedWorkerId = worker.id} />
                {#if w.skillProfile}
                  {@const sp = $skillProfiles.find(p => p.id === w.skillProfile)}
                  {#if sp}
                    <div class="skill-badge" title={sp.description}>{sp.icon} {sp.name}</div>
                  {/if}
                {/if}
                {#if w.parentName}
                  <div class="parent-link">
                    <span class="parent-arrow">&uarr;</span> Manager: {w.parentName}
                  </div>
                {/if}
                {#if tier !== 'consultant'}
                  <button class="nes-btn is-warning promote-btn" on:click|stopPropagation={() => handlePromote(w)}>
                    Promote
                  </button>
                {/if}
              </div>
            {/each}
            {#if tierWorkers.length === 0}
              <p class="empty-msg">No {tier}s</p>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </section>

  {#if selectedWorkerId}
    <WorkerDetailDrawer
      workerId={selectedWorkerId}
      onClose={() => selectedWorkerId = null}
      onSelectWorker={(id) => selectedWorkerId = id}
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

          <div class="nes-field">
            <label for="w-tier">Tier</label>
            <div class="nes-select is-dark">
              <select id="w-tier" bind:value={newTier}>
                {#each tierOptions as opt}
                  <option value={opt.id}>{opt.label}</option>
                {/each}
              </select>
            </div>
          </div>

          <div class="nes-field">
            <label for="w-parent">Parent (Manager)</label>
            <div class="nes-select is-dark">
              <select id="w-parent" bind:value={newParentID}>
                <option value="">None</option>
                {#each managers as m}
                  <option value={m.id}>{m.name} ({m.tier})</option>
                {/each}
              </select>
            </div>
          </div>

          <div class="nes-field">
            <label for="w-cli">CLI Tool</label>
            <div class="nes-select is-dark">
              <select id="w-cli" bind:value={newCliTool}>
                {#each cliToolOptions as opt}
                  <option value={opt.id}>{opt.label}</option>
                {/each}
              </select>
            </div>
          </div>

          <div class="nes-field">
            <label for="w-skill">Skill Profile</label>
            <div class="nes-select is-dark">
              <select id="w-skill" bind:value={newSkillProfile}>
                <option value="">None</option>
                {#each $skillProfiles as sp}
                  <option value={sp.id}>{sp.icon} {sp.name}</option>
                {/each}
              </select>
            </div>
            {#if newSkillProfile}
              {@const selected = $skillProfiles.find(sp => sp.id === newSkillProfile)}
              {#if selected}
                <p class="skill-desc">{selected.description}</p>
              {/if}
            {/if}
          </div>

          <div class="nes-field">
            <label for="w-backend">Backend ID (optional)</label>
            <input type="text" id="w-backend" class="nes-input is-dark" bind:value={newBackendID} placeholder="e.g. gpt-4" />
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

  .hierarchy-grid {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 12px;
    min-height: 200px;
  }

  .tier-column {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .tier-header {
    font-size: 11px;
    color: var(--accent-blue);
    border-bottom: 2px solid var(--border-color);
    padding-bottom: 6px;
    margin: 0;
  }

  .tier-workers {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .worker-wrap {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .parent-link {
    font-size: 9px;
    color: var(--text-secondary);
    text-align: center;
  }

  .parent-arrow {
    color: var(--accent-blue);
  }

  .promote-btn {
    font-size: 8px;
    padding: 2px 6px;
    align-self: center;
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

  .nes-field input[type="text"] {
    font-size: 10px;
    width: 100%;
  }

  .nes-field select {
    font-size: 10px;
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

  .skill-badge {
    font-size: 8px;
    text-align: center;
    color: var(--accent-green);
    background: rgba(0, 255, 65, 0.08);
    border: 1px solid rgba(0, 255, 65, 0.2);
    padding: 1px 6px;
    border-radius: 2px;
  }

  .skill-desc {
    font-size: 8px;
    color: var(--text-secondary);
    margin: 4px 0 0;
  }
</style>
