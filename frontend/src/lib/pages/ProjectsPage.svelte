<script>
  import { onMount } from 'svelte'
  import { projects, loadProjects, deleteProject } from '../stores/projects.js'
  import ProjectForm from '../components/ProjectForm.svelte'
  import AIChatProjectCreator from '../components/AIChatProjectCreator.svelte'
  import { addError } from '../stores/errors.js'
  import { t } from '../stores/i18n.js'

  export let onNavigate = () => {}

  let showForm = false
  let showAIChat = false
  let deleteConfirm = null

  onMount(async () => {
    try {
      await loadProjects()
    } catch (e) {
      addError('Failed to load projects: ' + e.message)
    }
  })

  function statusClass(status) {
    switch (status) {
      case 'active': return 'is-primary'
      case 'completed': return 'is-success'
      case 'archived': return 'is-disabled'
      default: return ''
    }
  }

  function handleDeleteClick(e, proj) {
    e.stopPropagation()
    deleteConfirm = proj
  }

  async function confirmDelete() {
    if (!deleteConfirm) return
    try {
      await deleteProject(deleteConfirm.id)
    } catch (e) {
      addError('Failed to delete project: ' + e.message)
    }
    deleteConfirm = null
  }

  function cancelDelete() {
    deleteConfirm = null
  }
</script>

<div class="projects-page">
  <section class="nes-container with-title is-dark">
    <p class="title">{$t('projects.title')}</p>
    <div class="toolbar">
      <button class="nes-btn is-primary" on:click={() => showForm = true}>{$t('projects.newProject')}</button>
      <button class="nes-btn is-warning" on:click={() => showAIChat = true}>{$t('projects.aiCreate')}</button>
    </div>

    <div class="projects-grid">
      {#each $projects as proj}
        <div
          class="nes-container is-rounded project-card"
          on:click={() => onNavigate('board', proj.id)}
          on:keydown={(e) => e.key === 'Enter' && onNavigate('board', proj.id)}
          role="button"
          tabindex="0"
        >
          <div class="card-header">
            <span class="proj-name">{proj.name}</span>
            <div class="card-header-right">
              <span class="nes-badge"><span class={statusClass(proj.status)}>{proj.status}</span></span>
              <button
                class="delete-btn"
                on:click={(e) => handleDeleteClick(e, proj)}
                on:keydown|stopPropagation
                title="Delete project"
              >x</button>
            </div>
          </div>
          <div class="card-body">
            {#if proj.description}
              <p class="proj-desc">{proj.description}</p>
            {/if}
            <div class="card-info">
              <span class="label">{$t('projects.repo')}</span>
              <span class="truncate">{proj.repoPath}</span>
            </div>
            <div class="card-info">
              <span class="label">{$t('projects.branch')}</span>
              <span>{proj.baseBranch}</span>
            </div>
            {#if proj.goals && proj.goals.length > 0}
              <div class="card-goals">
                <span class="label">{$t('projects.goals')}</span>
                {#each proj.goals as goal}
                  <span class="goal-tag">{goal}</span>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/each}
      {#if $projects.length === 0}
        <p class="empty-msg">{$t('projects.empty')}</p>
      {/if}
    </div>
  </section>

  <ProjectForm visible={showForm} onClose={() => showForm = false} />
  <AIChatProjectCreator visible={showAIChat} onClose={() => showAIChat = false} />

  {#if deleteConfirm}
    <div class="dialog-overlay" on:click={cancelDelete} on:keydown={(e) => e.key === 'Escape' && cancelDelete()} role="presentation">
      <div class="nes-dialog is-dark is-rounded" on:click|stopPropagation role="presentation">
        <p class="dialog-title">{$t('projects.deleteTitle')}</p>
        <p class="dialog-text">{$t('projects.deleteConfirm')} <strong>{deleteConfirm.name}</strong> {$t('projects.deleteConfirmSuffix')}</p>
        <menu class="dialog-menu">
          <button class="nes-btn is-error" on:click={confirmDelete}>{$t('projects.delete')}</button>
          <button class="nes-btn" on:click={cancelDelete}>{$t('common.cancel')}</button>
        </menu>
      </div>
    </div>
  {/if}
</div>

<style>
  .projects-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
  }

  .toolbar {
    margin-bottom: 12px;
    display: flex;
    gap: 8px;
  }

  .toolbar button {
    font-size: 10px;
  }

  .projects-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
  }

  .project-card {
    cursor: pointer;
    padding: 12px !important;
    margin: 0 !important;
    min-width: 280px;
    max-width: 360px;
    transition: border-color 0.1s;
  }

  .project-card:hover {
    border-color: var(--accent-blue) !important;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .card-header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .delete-btn {
    background: none;
    border: 2px solid var(--text-secondary);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 10px;
    padding: 0 4px;
    line-height: 1.2;
    font-family: inherit;
    transition: border-color 0.1s, color 0.1s;
  }

  .delete-btn:hover {
    border-color: #e76e55;
    color: #e76e55;
  }

  .proj-name {
    font-size: 12px;
    color: var(--accent-green);
  }

  .proj-desc {
    font-size: 9px;
    color: var(--text-secondary);
    margin: 4px 0;
  }

  .card-body {
    font-size: 9px;
  }

  .card-info {
    margin: 4px 0;
  }

  .label {
    color: var(--text-secondary);
  }

  .truncate {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: inline-block;
    vertical-align: bottom;
  }

  .card-goals {
    margin-top: 6px;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
  }

  .goal-tag {
    font-size: 8px;
    padding: 2px 6px;
    border: 2px solid var(--accent-blue);
    display: inline-block;
  }

  .empty-msg {
    color: var(--text-secondary);
    font-size: 10px;
  }

  /* Delete confirmation dialog */
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
    max-width: 400px;
    width: 90%;
    padding: 24px !important;
  }

  .dialog-title {
    color: #e76e55;
    margin-bottom: 12px;
    font-size: 12px;
  }

  .dialog-text {
    font-size: 10px;
    margin-bottom: 16px;
  }

  .dialog-menu {
    display: flex;
    gap: 12px;
    justify-content: flex-end;
    padding: 0;
    margin: 0;
  }

  .dialog-menu button {
    font-size: 10px;
  }
</style>
