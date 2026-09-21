<script>
  import Toolbar from './Toolbar.svelte';
  import TOCSidebar from './TOCSidebar.svelte';
  import ChapterPane from './ChapterPane.svelte';
  import PDFPane from './PDFPane.svelte';
  import { tocOpen, currentBook, currentBookId, highlights, bookmarks } from '../stores/app.js';
  import { getHighlights, getBookmarks } from './api.js';

  $: if ($currentBookId) {
    getHighlights($currentBookId).then((items) => {
      highlights.set(items || []);
    });
    getBookmarks($currentBookId).then((items) => {
      bookmarks.set(items || []);
    });
  }
</script>

<div class="reader-view">
  <Toolbar />
  <div class="reader-body">
    {#if $tocOpen}
      <TOCSidebar />
    {/if}
    {#if $currentBook?.format === 'pdf'}
      <PDFPane />
    {:else}
      <ChapterPane />
    {/if}
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
