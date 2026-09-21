<script>
  import { onMount, onDestroy } from 'svelte';
  import {
    searchOpen, currentBookId, currentBook, currentSpineIndex,
    currentChapter
  } from '../stores/app.js';
  import { searchBook, getChapter, saveProgress } from './api.js';

  let inputEl;
  let query = '';
  let searching = false;
  let results = [];
  let selectedIndex = 0;
  let debounceTimer;

  $: totalMatches = results.reduce((acc, r) => acc + (r.matchCount || 0), 0);

  function handleInput() {
    clearTimeout(debounceTimer);
    if (!query.trim()) {
      results = [];
      searching = false;
      return;
    }
    searching = true;
    debounceTimer = setTimeout(performSearch, 200);
  }

  async function performSearch() {
    if (!query.trim() || !$currentBookId) {
      searching = false;
      return;
    }
    try {
      const res = await searchBook($currentBookId, query.trim());
      results = res || [];
      selectedIndex = 0;
    } finally {
      searching = false;
    }
  }

  async function jumpToResult(r) {
    const bookId = $currentBookId;
    if (!bookId) return;

    searchOpen.set(false);

    if ($currentSpineIndex !== r.spineIndex) {
      currentSpineIndex.set(r.spineIndex);
      const html = await getChapter(bookId, r.spineIndex);
      currentChapter.set(html);
      saveProgress(bookId, r.spineIndex, 0);
    }

    setTimeout(() => {
      const iframe = document.querySelector('.chapter-frame');
      if (iframe?.contentWindow) {
        iframe.contentWindow.postMessage({
          type: 'find-and-scroll',
          query: query.trim()
        }, '*');
      }
    }, 150);
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      searchOpen.set(false);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (results.length > 0) {
        selectedIndex = (selectedIndex + 1) % results.length;
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (results.length > 0) {
        selectedIndex = (selectedIndex - 1 + results.length) % results.length;
      }
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (results[selectedIndex]) {
        jumpToResult(results[selectedIndex]);
      } else {
        performSearch();
      }
    }
  }

  onMount(() => {
    inputEl?.focus();
  });
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events -->
<div class="search-backdrop" on:click|self={() => searchOpen.set(false)} role="dialog" aria-modal="true">
  <div class="search-modal">
    <div class="search-input-wrapper">
      <svg class="search-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
      </svg>
      <input
        bind:this={inputEl}
        bind:value={query}
        on:input={handleInput}
        type="text"
        class="search-input"
        placeholder="Search entire book..."
        aria-label="Search"
      />
      {#if searching}
        <div class="search-spinner"></div>
      {:else if query}
        <button class="clear-btn" on:click={() => { query = ''; results = []; inputEl?.focus(); }}>
          ✕
        </button>
      {/if}
    </div>

    {#if query.trim()}
      <div class="search-header">
        <span class="search-summary">
          {#if searching}
            Searching...
          {:else if results.length === 0}
            No results found for “{query}”
          {:else}
            {totalMatches} match{totalMatches === 1 ? '' : 'es'} across {results.length} chapter{results.length === 1 ? '' : 's'}
          {/if}
        </span>
      </div>
    {/if}

    <div class="search-results">
      {#each results as r, i}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <div
          class="result-item"
          class:selected={i === selectedIndex}
          on:click={() => jumpToResult(r)}
          on:mouseenter={() => selectedIndex = i}
        >
          <div class="result-header">
            <span class="result-chapter">{r.chapterTitle}</span>
            <span class="result-count">{r.matchCount} match{r.matchCount === 1 ? '' : 'es'}</span>
          </div>
          <p class="result-snippet">
            {r.snippet}
          </p>
        </div>
      {/each}
    </div>

    <div class="search-footer">
      <span class="shortcut-tip"><kbd>↑</kbd><kbd>↓</kbd> navigate</span>
      <span class="shortcut-tip"><kbd>↵</kbd> select</span>
      <span class="shortcut-tip"><kbd>esc</kbd> dismiss</span>
    </div>
  </div>
</div>

<style>
  .search-backdrop {
    position: fixed;
    inset: 0;
    z-index: 2000;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 10vh;
    animation: fadeIn 0.15s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .search-modal {
    width: 100%;
    max-width: 620px;
    background: var(--bg-card);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-lg);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.6), 0 4px 16px rgba(0, 0, 0, 0.2);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slideDown 0.18s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes slideDown {
    from { transform: translateY(-12px) scale(0.98); opacity: 0; }
    to { transform: translateY(0) scale(1); opacity: 1; }
  }

  .search-input-wrapper {
    display: flex;
    align-items: center;
    padding: 14px 18px;
    background: var(--bg-card);
    border-bottom: 1px solid var(--border-subtle);
    gap: 12px;
  }

  .search-icon {
    color: var(--fg-tertiary);
    flex-shrink: 0;
  }

  .search-input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    font-size: 1.0625rem;
    font-family: var(--font-sans);
    color: var(--fg-primary);
  }

  .search-input::placeholder {
    color: var(--fg-tertiary);
  }

  .clear-btn {
    border: none;
    background: transparent;
    color: var(--fg-tertiary);
    font-size: 14px;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .clear-btn:hover {
    color: var(--fg-primary);
    background: var(--bg-hover);
  }

  .search-spinner {
    width: 18px;
    height: 18px;
    border: 2px solid var(--border-subtle);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .search-header {
    padding: 8px 18px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-subtle);
  }

  .search-summary {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--fg-secondary);
  }

  .search-results {
    max-height: 420px;
    overflow-y: auto;
    padding: 8px;
    background: var(--bg-card);
  }

  .result-item {
    padding: 10px 14px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out);
    margin-bottom: 4px;
  }

  .result-item:hover, .result-item.selected {
    background: var(--bg-hover);
  }

  .result-item.selected {
    background: var(--accent-subtle);
  }

  .result-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 4px;
  }

  .result-chapter {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--accent);
  }

  .result-count {
    font-size: 0.6875rem;
    color: var(--fg-tertiary);
    background: var(--bg-primary);
    padding: 2px 6px;
    border-radius: 10px;
    border: 1px solid var(--border-subtle);
  }

  .result-snippet {
    font-size: 0.8125rem;
    line-height: 1.45;
    color: var(--fg-secondary);
    margin: 0;
  }

  .search-footer {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 10px 18px;
    background: var(--bg-secondary);
    border-top: 1px solid var(--border-subtle);
    font-size: 0.6875rem;
    color: var(--fg-tertiary);
  }

  .shortcut-tip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  kbd {
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    padding: 1px 5px;
    font-family: inherit;
    font-size: 0.625rem;
  }
</style>
