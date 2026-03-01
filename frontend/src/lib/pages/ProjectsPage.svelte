<script>
  import { onMount } from 'svelte'
  import { projects, loadProjects } from '../stores/projects.js'
  import ProjectForm from '../components/ProjectForm.svelte'
  import { addError } from '../stores/errors.js'

  export let onNavigate = () => {}

  let showForm = false

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
</script>

<div class="projects-page">
  <section class="nes-container with-title is-dark">
    <p class="title">Projects</p>
    <div class="toolbar">
      <button class="nes-btn is-primary" on:click={() => showForm = true}>+ New Project</button>
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
            <span class="nes-badge"><span class={statusClass(proj.status)}>{proj.status}</span></span>
          </div>
          <div class="card-body">
            {#if proj.description}
              <p class="proj-desc">{proj.description}</p>
            {/if}
            <div class="card-info">
              <span class="label">repo:</span>
              <span class="truncate">{proj.repoPath}</span>
            </div>
            <div class="card-info">
              <span class="label">branch:</span>
              <span>{proj.baseBranch}</span>
            </div>
            {#if proj.goals && proj.goals.length > 0}
              <div class="card-goals">
                <span class="label">goals:</span>
                {#each proj.goals as goal}
                  <span class="goal-tag">{goal}</span>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/each}
      {#if $projects.length === 0}
        <p class="empty-msg">No projects yet. Create one to get started!</p>
      {/if}
    </div>
  </section>

  <ProjectForm visible={showForm} onClose={() => showForm = false} />
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
</style>
