<script>
  import RetroPanel from '../components/RetroPanel.svelte'
  import { t } from '../stores/i18n.js'
</script>

<div class="retro-page">
  <div class="nes-container is-dark with-title">
    <p class="title">{$t('retro.title')}</p>
    <RetroPanel />
  </div>
</div>

<style>
  .retro-page {
    padding: 8px;
    height: 100%;
    overflow-y: auto;
  }
</style>
