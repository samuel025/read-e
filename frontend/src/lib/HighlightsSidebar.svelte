<script>
  import {
    highlights, currentBookId, currentSpineIndex,
    currentChapter, toc
  } from '../stores/app.js';
  import { getHighlights, deleteHighlight, getChapter, saveProgress } from './api.js';

  let selectedColor = 'all';

  const colors = [
    { id: 'all', label: 'All', hex: 'var(--fg-tertiary)' },
    { id: 'yellow', label: 'Yellow', hex: '#eab308' },
    { id: 'green', label: 'Green', hex: '#22c55e' },
    { id: 'blue', label: 'Blue', hex: '#3b82f6' },
    { id: 'purple', label: 'Purple', hex: '#a855f7' },
    { id: 'pink', label: 'Pink', hex: '#f43f5e' },
  ];

  $: filteredHighlights = $highlights.filter(h => {
    if (selectedColor === 'all') return true;
    return h.color === selectedColor;
  });

  function getChapterTitle(spineIndex) {
    function findTitle(entries) {
      for (const entry of entries) {
        if (entry.spineIndex === spineIndex) return entry.title;
        if (entry.children?.length) {
          const childTitle = findTitle(entry.children);
          if (childTitle) return childTitle;
        }
      }
      return null;
    }
    return findTitle($toc) || `Chapter ${spineIndex + 1}`;
  }

  function formatDate(d) {
    if (!d) return '';
    try {
      const date = new Date(d);
      return date.toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
      });
    } catch {
      return '';
    }
  }

  async function jumpToHighlight(h) {
    const bookId = $currentBookId;
    if (!bookId) return;

    if ($currentSpineIndex !== h.spineIndex) {
      currentSpineIndex.set(h.spineIndex);
      const html = await getChapter(bookId, h.spineIndex);
      currentChapter.set(html);
      saveProgress(bookId, h.spineIndex, 0);
    }

    // Trigger iframe scroll & highlight pulse after render
    setTimeout(() => {
      const iframe = document.querySelector('.chapter-frame');
      if (iframe?.contentWindow) {
        iframe.contentWindow.postMessage({
          type: 'jump-to-highlight',
          highlightId: h.id,
        }, '*');
      }
    }, 150);
  }

  async function handleDelete(e, id) {
    e.stopPropagation();
    await deleteHighlight(id);
    highlights.update(items => items.filter(item => item.id !== id));

    // Also notify iframe to unwrap mark
    const iframe = document.querySelector('.chapter-frame');
    if (iframe?.contentWindow) {
      iframe.contentWindow.postMessage({
        type: 'remove-highlight-mark',
        highlightId: id,
      }, '*');
    }
  }
</script>

<div class="highlights-panel">
  <!-- Color filter pills -->
  <div class="filter-bar">
    {#each colors as c}
      <button
        class="filter-chip"
        class:active={selectedColor === c.id}
        on:click={() => selectedColor = c.id}
        title="Filter by {c.label}"
      >
        {#if c.id !== 'all'}
          <span class="color-dot" style="background: {c.hex};"></span>
        {/if}
        <span>{c.label}</span>
      </button>
    {/each}
  </div>

  <!-- Highlights list -->
  <div class="highlights-list">
    {#if filteredHighlights.length === 0}
      <div class="empty-state">
        <div class="empty-icon">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="m14 12-8.5 8.5a2.12 2.12 0 0 1-3-3L11 9"/>
            <path d="M16 10l3.5-3.5a2.12 2.12 0 0 0 0-3 2.12 2.12 0 0 0-3 0L13 7"/>
            <path d="m17 7 3 3"/>
          </svg>
        </div>
        <p class="empty-title">
          {selectedColor === 'all' ? 'No highlights yet' : `No ${selectedColor} highlights`}
        </p>
        <p class="empty-desc">
          Select any text while reading to highlight passages in your choice of color.
        </p>
      </div>
    {:else}
      <div class="highlight-cards">
        {#each filteredHighlights as h (h.id)}
          <div
            class="highlight-card color-{h.color}"
            on:click={() => jumpToHighlight(h)}
            role="button"
            tabindex="0"
            on:keydown={(e) => e.key === 'Enter' && jumpToHighlight(h)}
          >
            <div class="card-header">
              <span class="chapter-badge">
                <span class="dot-indicator color-{h.color}"></span>
                {getChapterTitle(h.spineIndex)}
              </span>
              <div class="header-right">
                <span class="date-badge">{formatDate(h.createdAt)}</span>
                <button
                  class="delete-btn"
                  on:click={(e) => handleDelete(e, h.id)}
                  title="Delete highlight"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/>
                    <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>
                  </svg>
                </button>
              </div>
            </div>
            <p class="highlight-quote">“{h.text}”</p>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .highlights-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .filter-bar {
    display: flex;
    gap: 4px;
    padding: 8px 10px;
    overflow-x: auto;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }

  .filter-bar::-webkit-scrollbar {
    display: none;
  }

  .filter-chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px;
    font-size: 0.6875rem;
    font-weight: 500;
    color: var(--fg-secondary);
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    cursor: pointer;
    white-space: nowrap;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .filter-chip:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .filter-chip.active {
    background: var(--accent-subtle);
    color: var(--accent);
    border-color: var(--accent);
    font-weight: 600;
  }

  .color-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    display: inline-block;
  }

  .highlights-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-sm);
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px 16px;
    text-align: center;
    color: var(--fg-tertiary);
  }

  .empty-icon {
    margin-bottom: var(--space-sm);
    opacity: 0.5;
  }

  .empty-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--fg-secondary);
    margin-bottom: 4px;
  }

  .empty-desc {
    font-size: 0.75rem;
    line-height: 1.4;
    max-width: 200px;
  }

  .highlight-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .highlight-card {
    padding: 10px 12px;
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-left-width: 4px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: transform var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .highlight-card:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    border-color: var(--border-medium);
  }

  .highlight-card.color-yellow { border-left-color: #eab308; }
  .highlight-card.color-green  { border-left-color: #22c55e; }
  .highlight-card.color-blue   { border-left-color: #3b82f6; }
  .highlight-card.color-purple { border-left-color: #a855f7; }
  .highlight-card.color-pink   { border-left-color: #f43f5e; }

  .dot-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    display: inline-block;
  }
  .dot-indicator.color-yellow { background: #eab308; }
  .dot-indicator.color-green  { background: #22c55e; }
  .dot-indicator.color-blue   { background: #3b82f6; }
  .dot-indicator.color-purple { background: #a855f7; }
  .dot-indicator.color-pink   { background: #f43f5e; }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 6px;
    font-size: 0.6875rem;
  }

  .chapter-badge {
    display: flex;
    align-items: center;
    gap: 5px;
    color: var(--fg-secondary);
    font-weight: 500;
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .date-badge {
    color: var(--fg-tertiary);
    font-size: 0.6875rem;
  }

  .delete-btn {
    border: none;
    background: transparent;
    color: var(--fg-tertiary);
    padding: 2px;
    border-radius: 4px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color var(--duration-fast) var(--ease-out),
                background var(--duration-fast) var(--ease-out);
  }

  .delete-btn:hover {
    color: #ef4444;
    background: rgba(239, 68, 68, 0.1);
  }

  .highlight-quote {
    font-size: 0.8125rem;
    line-height: 1.45;
    color: var(--fg-primary);
    font-style: italic;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
    margin: 0;
  }
</style>
