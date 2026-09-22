<script>
  import Toolbar from './Toolbar.svelte';
  import TOCSidebar from './TOCSidebar.svelte';
  import ChapterPane from './ChapterPane.svelte';
  import PDFPane from './PDFPane.svelte';
  import { tocOpen, currentBook, currentBookId, highlights, bookmarks, currentSpineIndex, spineCount, library } from '../stores/app.js';
  import { getHighlights, getBookmarks } from './api.js';

  import { onMount, onDestroy } from 'svelte';
  import { logReadingSession, markBookFinished } from './api.js';

  $: if ($currentBookId) {
    getHighlights($currentBookId).then((items) => {
      highlights.set(items || []);
    });
    getBookmarks($currentBookId).then((items) => {
      bookmarks.set(items || []);
    });
  }

  // Reading Tracker Logic
  let activeSeconds = 0;
  let pagesTurned = 0;
  let lastActive = Date.now();
  let flushInterval;

  function markActive() {
    lastActive = Date.now();
  }

  onMount(() => {
    // Flush to DB every 60 seconds
    flushInterval = setInterval(async () => {
      if (activeSeconds > 0) {
        const today = new Date().toISOString().split('T')[0];
        try {
          await logReadingSession(today, activeSeconds, pagesTurned);
          activeSeconds = 0;
          pagesTurned = 0;
        } catch(e) {}
      }
    }, 60000);

    // Track active reading time every second
    const activeInterval = setInterval(() => {
      if ($currentBookId && Date.now() - lastActive < 120000) { // Idle after 2 mins
        activeSeconds++;
      }
    }, 1000);

    return () => {
      clearInterval(flushInterval);
      clearInterval(activeInterval);
    };
  });

  // Track page turns / spine changes
  let prevSpine = null;
  $: if ($currentSpineIndex !== prevSpine) {
    if (prevSpine !== null) {
      pagesTurned++;
      markActive();
    }
    prevSpine = $currentSpineIndex;
  }

  // Auto-detect finish when user reaches the last chapter/page
  $: {
    if ($currentBookId && $spineCount > 1 && $currentSpineIndex !== null && $currentSpineIndex >= $spineCount - 1) {
      markBookFinished($currentBookId, true).catch(() => {});
      library.update(lib => lib.map(b => b.id === $currentBookId ? { ...b, finished: true, progress: 100 } : b));
    }
  }
</script>

<div class="reader-view">
  <Toolbar />
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div 
    class="reader-body"
    on:mousemove={markActive}
    on:mousedown={markActive}
    on:wheel={markActive}
    on:keydown={markActive}
  >
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
