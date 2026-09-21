<script>
  import { toc, currentSpineIndex, currentBookId, currentChapter } from '../stores/app.js';
  import { getChapter, saveProgress } from './api.js';

  async function navigateTo(spineIndex) {
    const bookId = $currentBookId;
    if (!bookId) return;

    currentSpineIndex.set(spineIndex);

    const html = await getChapter(bookId, spineIndex);
    currentChapter.set(html);

    // Save progress
    saveProgress(bookId, spineIndex, 0);
  }

  function renderEntries(entries, depth = 0) {
    return entries;
  }
</script>

<aside class="toc-sidebar">
  <div class="toc-header">
    <h2 class="toc-title">Contents</h2>
  </div>
  <nav class="toc-list" aria-label="Table of contents">
    {#if $toc.length === 0}
      <p class="toc-empty">No table of contents available</p>
    {:else}
      <ul class="toc-entries">
        {#each $toc as entry, i}
          <li>
            <button
              class="toc-entry"
              class:active={$currentSpineIndex === entry.spineIndex}
              on:click={() => navigateTo(entry.spineIndex)}
            >
              <span class="toc-entry-title">{entry.title}</span>
            </button>
            {#if entry.children && entry.children.length > 0}
              <ul class="toc-children">
                {#each entry.children as child}
                  <li>
                    <button
                      class="toc-entry toc-child"
                      class:active={$currentSpineIndex === child.spineIndex}
                      on:click={() => navigateTo(child.spineIndex)}
                    >
                      <span class="toc-entry-title">{child.title}</span>
                    </button>
                  </li>
                {/each}
              </ul>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </nav>
</aside>

<style>
  .toc-sidebar {
    width: var(--sidebar-width);
    min-width: var(--sidebar-width);
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border-subtle);
    overflow: hidden;
  }

  .toc-header {
    padding: var(--space-md) var(--space-md);
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }

  .toc-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--fg-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .toc-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-sm);
  }

  .toc-empty {
    padding: var(--space-md);
    font-size: 0.8125rem;
    color: var(--fg-tertiary);
    text-align: center;
  }

  .toc-entries {
    list-style: none;
  }

  .toc-children {
    list-style: none;
    padding-left: var(--space-md);
  }

  .toc-entry {
    display: block;
    width: 100%;
    text-align: left;
    padding: 6px 10px;
    margin: 1px 0;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--fg-secondary);
    font-family: var(--font-sans);
    font-size: 0.8125rem;
    line-height: 1.4;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
  }

  .toc-entry:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .toc-entry.active {
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 500;
  }

  .toc-entry:active {
    transform: scale(0.98);
  }

  .toc-child {
    font-size: 0.75rem;
  }

  .toc-entry-title {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
