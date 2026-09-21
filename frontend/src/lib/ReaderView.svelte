<script>
  import Toolbar from './Toolbar.svelte';
  import TOCSidebar from './TOCSidebar.svelte';
  import ChapterPane from './ChapterPane.svelte';
  import { tocOpen } from '../stores/app.js';
</script>

<div class="reader-view">
  <Toolbar />
  <div class="reader-body">
    {#if $tocOpen}
      <TOCSidebar />
    {/if}
    <ChapterPane />
  </div>
</div>

<style>
  .reader-view {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-primary);
  }

  .reader-body {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
</style>
