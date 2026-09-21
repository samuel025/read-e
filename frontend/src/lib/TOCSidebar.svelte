<script>
  import {
    toc, currentSpineIndex, currentBookId, currentBook, currentChapter,
    activeSidebarTab, highlights, bookmarks
  } from '../stores/app.js';
  import { getChapter, saveProgress, getHighlights, getBookmarks } from './api.js';
  import HighlightsSidebar from './HighlightsSidebar.svelte';
  import BookmarksSidebar from './BookmarksSidebar.svelte';

  let selectedKey = null;

  function getEntryKey(item) {
    if (!item) return '';
    return item.href || `${item.spineIndex}_${item.title}`;
  }

  function determineActiveKey(tocList, spineIdx, userSelectedKey) {
    if (!tocList || tocList.length === 0 || spineIdx === undefined) return null;

    let hasSelectedMatchingSpine = false;
    function checkSelected(entries) {
      for (const e of entries) {
        if (getEntryKey(e) === userSelectedKey && e.spineIndex === spineIdx) {
          hasSelectedMatchingSpine = true;
          return;
        }
        if (e.children && e.children.length > 0) {
          checkSelected(e.children);
          if (hasSelectedMatchingSpine) return;
        }
      }
    }
    if (userSelectedKey) {
      checkSelected(tocList);
      if (hasSelectedMatchingSpine) return userSelectedKey;
    }

    let firstMatch = null;
    function findFirst(entries) {
      for (const e of entries) {
        if (e.spineIndex === spineIdx) {
          firstMatch = getEntryKey(e);
          return;
        }
        if (e.children && e.children.length > 0) {
          findFirst(e.children);
          if (firstMatch) return;
        }
      }
    }
    findFirst(tocList);
    return firstMatch;
  }

  $: activeKey = determineActiveKey($toc, $currentSpineIndex, selectedKey);

  async function navigateTo(entry) {
    const bookId = $currentBookId;
    if (!bookId || !entry) return;

    selectedKey = getEntryKey(entry);
    const spineIndex = entry.spineIndex;

    if ($currentBook?.format === 'pdf') {
      window.dispatchEvent(new CustomEvent('pdf-scroll-to-toc', {
        detail: {
          pageIndex: spineIndex,
          title: entry.title,
          href: entry.href
        }
      }));
      return;
    }

    const hash = entry.href && entry.href.includes('#') ? entry.href.split('#')[1] : null;

    if ($currentSpineIndex === spineIndex) {
      if (hash) {
        window.dispatchEvent(new CustomEvent('epub-scroll-to-hash', {
          detail: { hash }
        }));
      } else {
        window.dispatchEvent(new CustomEvent('epub-scroll-to-top'));
      }
      return;
    }

    currentSpineIndex.set(spineIndex);
    const html = await getChapter(bookId, spineIndex);
    currentChapter.set(html);

    if (hash) {
      setTimeout(() => {
        window.dispatchEvent(new CustomEvent('epub-scroll-to-hash', {
          detail: { hash }
        }));
      }, 150);
    } else {
      saveProgress(bookId, spineIndex, 0);
    }
  }

  $: if ($currentBookId) {
    getHighlights($currentBookId).then((items) => {
      highlights.set(items || []);
    });
    getBookmarks($currentBookId).then((items) => {
      bookmarks.set(items || []);
    });
  }
</script>

<aside class="toc-sidebar">
  <div class="sidebar-header">
    <div class="segmented-control" role="tablist">
      <button
        class="tab-btn"
        class:active={$activeSidebarTab === 'toc'}
        on:click={() => activeSidebarTab.set('toc')}
        role="tab"
        aria-selected={$activeSidebarTab === 'toc'}
      >
        Contents
      </button>
      <button
        class="tab-btn"
        class:active={$activeSidebarTab === 'highlights'}
        on:click={() => activeSidebarTab.set('highlights')}
        role="tab"
        aria-selected={$activeSidebarTab === 'highlights'}
      >
        Highlights
        {#if $highlights.length > 0}
          <span class="count-badge">{$highlights.length}</span>
        {/if}
      </button>
      <button
        class="tab-btn"
        class:active={$activeSidebarTab === 'bookmarks'}
        on:click={() => activeSidebarTab.set('bookmarks')}
        role="tab"
        aria-selected={$activeSidebarTab === 'bookmarks'}
      >
        Marks
        {#if $bookmarks.length > 0}
          <span class="count-badge">{$bookmarks.length}</span>
        {/if}
      </button>
    </div>
  </div>

  {#if $activeSidebarTab === 'toc'}
    <nav class="toc-list" aria-label="Table of contents">
      {#if $toc.length === 0}
        <p class="toc-empty">No table of contents available</p>
      {:else}
        <ul class="toc-entries">
          {#each $toc as entry}
            <li>
              <button
                class="toc-entry"
                class:active={activeKey === getEntryKey(entry)}
                on:click={() => navigateTo(entry)}
              >
                <span class="toc-entry-title">{entry.title}</span>
              </button>
              {#if entry.children && entry.children.length > 0}
                <ul class="toc-children">
                  {#each entry.children as child}
                    <li>
                      <button
                        class="toc-entry toc-child"
                        class:active={activeKey === getEntryKey(child)}
                        on:click={() => navigateTo(child)}
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
  {:else if $activeSidebarTab === 'highlights'}
    <HighlightsSidebar />
  {:else}
    <BookmarksSidebar />
  {/if}
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

  .sidebar-header {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }

  .segmented-control {
    display: flex;
    background: var(--bg-primary);
    border-radius: var(--radius-sm);
    padding: 2px;
    border: 1px solid var(--border-subtle);
  }

  .tab-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 5px 8px;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--fg-secondary);
    background: transparent;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .tab-btn:hover {
    color: var(--fg-primary);
  }

  .tab-btn.active {
    background: var(--bg-card);
    color: var(--fg-primary);
    font-weight: 600;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  }

  .count-badge {
    background: var(--accent);
    color: white;
    font-size: 0.625rem;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 10px;
    line-height: 1.2;
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
    margin: 0;
    padding: 0;
  }

  .toc-children {
    list-style: none;
    padding-left: var(--space-md);
    margin: 0;
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
