<script>
  import {
    bookmarks, currentBookId, currentSpineIndex,
    currentChapter, toc
  } from '../stores/app.js';
  import { deleteBookmark, getChapter, saveProgress } from './api.js';

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

  async function jumpToBookmark(bm) {
    const bookId = $currentBookId;
    if (!bookId) return;

    const switched = $currentSpineIndex !== bm.spineIndex;
    if (switched) {
      currentSpineIndex.set(bm.spineIndex);
      const html = await getChapter(bookId, bm.spineIndex);
      currentChapter.set(html);
    }

    saveProgress(bookId, bm.spineIndex, bm.scrollOffset);

    const sendScroll = () => {
      const iframe = document.querySelector('.chapter-frame');
      if (iframe?.contentWindow) {
        iframe.contentWindow.postMessage({
          type: 'scroll-to-offset',
          offset: bm.scrollOffset
        }, '*');
      }
    };

    setTimeout(sendScroll, switched ? 350 : 30);
    if (switched) {
      setTimeout(sendScroll, 750);
    }
  }

  async function handleDelete(e, id) {
    e.stopPropagation();
    await deleteBookmark(id);
    bookmarks.update(items => items.filter(item => item.id !== id));
  }
</script>

<div class="bookmarks-panel">
  <div class="bookmarks-list">
    {#if $bookmarks.length === 0}
      <div class="empty-state">
        <div class="empty-icon">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>
          </svg>
        </div>
        <p class="empty-title">No bookmarks yet</p>
        <p class="empty-desc">
          Click the bookmark ribbon in the toolbar or press <kbd>⌘D</kbd> to save your place.
        </p>
      </div>
    {:else}
      <div class="bookmark-cards">
        {#each $bookmarks as bm (bm.id)}
          <!-- svelte-ignore a11y-click-events-have-key-events -->
          <div
            class="bookmark-card"
            on:click={() => jumpToBookmark(bm)}
            role="button"
            tabindex="0"
          >
            <div class="card-header">
              <div class="chapter-info">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="var(--accent)" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>
                </svg>
                <span class="chapter-name">{getChapterTitle(bm.spineIndex)}</span>
              </div>
              <div class="header-right">
                <span class="date-badge">{formatDate(bm.createdAt)}</span>
                <button
                  class="delete-btn"
                  on:click={(e) => handleDelete(e, bm.id)}
                  title="Delete bookmark"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/>
                    <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>
                  </svg>
                </button>
              </div>
            </div>
            {#if bm.title}
              <p class="bookmark-title">{bm.title}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .bookmarks-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .bookmarks-list {
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

  kbd {
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    padding: 1px 4px;
    font-size: 0.6875rem;
  }

  .bookmark-cards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .bookmark-card {
    padding: 10px 12px;
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: transform var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .bookmark-card:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    border-color: var(--border-medium);
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.75rem;
  }

  .chapter-info {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 600;
    color: var(--fg-primary);
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

  .bookmark-title {
    margin: 6px 0 0 0;
    font-size: 0.75rem;
    color: var(--fg-secondary);
    line-height: 1.4;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
