<script>
  import {
    view, currentBook, currentBookId, currentSpineIndex,
    spineCount, tocOpen, settings, settingsOpen,
    activeSidebarTab, highlights, bookmarks, searchOpen, toc
  } from '../stores/app.js';
  import {
    saveSettings, saveBookmark, deleteBookmark,
    getChapterWordCount
  } from './api.js';

  let chapterMinutes = null;

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

  $: if ($currentBookId && $currentSpineIndex !== undefined) {
    loadReadingStats($currentBookId, $currentSpineIndex);
  }

  async function loadReadingStats(bookId, spineIndex) {
    try {
      const words = await getChapterWordCount(bookId, spineIndex);
      if (words > 0) {
        chapterMinutes = Math.max(1, Math.round(words / 220));
      } else {
        chapterMinutes = null;
      }
    } catch {
      chapterMinutes = null;
    }
  }

  $: currentBookmark = $bookmarks.find(b => b.spineIndex === $currentSpineIndex);
  $: isBookmarked = !!currentBookmark;

  async function toggleBookmark() {
    if (!$currentBookId) return;

    if (isBookmarked && currentBookmark) {
      await deleteBookmark(currentBookmark.id);
      bookmarks.update(items => items.filter(b => b.id !== currentBookmark.id));
    } else {
      const iframe = document.querySelector('.chapter-frame');
      const scrollY = iframe?.contentWindow?.scrollY || iframe?.contentWindow?.document?.documentElement?.scrollTop || 0;
      const title = getChapterTitle($currentSpineIndex);

      const newBm = {
        id: crypto.randomUUID ? crypto.randomUUID() : 'bm_' + Date.now(),
        bookId: $currentBookId,
        spineIndex: $currentSpineIndex,
        title: title,
        scrollOffset: Math.round(scrollY),
        createdAt: new Date().toISOString()
      };
      await saveBookmark(newBm);
      bookmarks.update(items => [newBm, ...items]);
    }
  }

  function goBack() {
    view.set('library');
    currentBook.set(null);
    currentBookId.set(null);
  }

  function toggleTOC() {
    if ($tocOpen && $activeSidebarTab === 'toc') {
      tocOpen.set(false);
    } else {
      tocOpen.set(true);
      activeSidebarTab.set('toc');
    }
  }

  function toggleHighlights() {
    if ($tocOpen && $activeSidebarTab === 'highlights') {
      tocOpen.set(false);
    } else {
      tocOpen.set(true);
      activeSidebarTab.set('highlights');
    }
  }

  function toggleBookmarks() {
    if ($tocOpen && $activeSidebarTab === 'bookmarks') {
      tocOpen.set(false);
    } else {
      tocOpen.set(true);
      activeSidebarTab.set('bookmarks');
    }
  }

  function openSearch() {
    searchOpen.set(true);
  }

  function openSettings() {
    settingsOpen.set(true);
  }

  async function changeFontSize(delta) {
    settings.update((s) => {
      const newSize = Math.max(0.6, Math.min(2.0, s.fontSize + delta));
      const updated = { ...s, fontSize: newSize };
      saveSettings(updated);
      return updated;
    });
  }

  async function cycleTheme() {
    settings.update((s) => {
      const themes = ['dark', 'light', 'sepia'];
      const idx = themes.indexOf(s.theme);
      const next = themes[(idx + 1) % themes.length];
      const updated = { ...s, theme: next };
      saveSettings(updated);
      return updated;
    });
  }

  const themeIcons = {
    dark: '🌙',
    light: '☀️',
    sepia: '📜',
  };
</script>

<header class="toolbar">
  <div class="toolbar-left">
    <button class="btn btn-ghost btn-icon" on:click={goBack} title="Back to library" id="back-btn">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>
      </svg>
    </button>

    <button class="btn btn-ghost btn-icon" class:active={$tocOpen && $activeSidebarTab === 'toc'} on:click={toggleTOC} title="Table of contents" id="toc-toggle-btn">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 12h18"/><path d="M3 6h18"/><path d="M3 18h18"/>
      </svg>
    </button>

    <button class="btn btn-ghost btn-icon" class:active={$tocOpen && $activeSidebarTab === 'highlights'} on:click={toggleHighlights} title="Highlights ({$highlights.length})" id="highlights-toggle-btn">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m9 11-6 6v3h3l6-6"/>
        <path d="m22 12-4.6 4.6a2 2 0 0 1-2.8 0l-5.2-5.2a2 2 0 0 1 0-2.8L14 4"/>
      </svg>
    </button>

    <button class="btn btn-ghost btn-icon" class:active={$tocOpen && $activeSidebarTab === 'bookmarks'} on:click={toggleBookmarks} title="Bookmarks ({$bookmarks.length})" id="bookmarks-tab-btn">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>
      </svg>
    </button>

    {#if $currentBook}
      <span class="toolbar-title">{$currentBook.title}</span>
    {/if}
  </div>

  <div class="toolbar-center">
    <span class="chapter-indicator">
      {$currentSpineIndex + 1} / {$spineCount}
    </span>
    <div class="progress-bar-container">
      <div
        class="progress-bar-fill"
        style="width: {$spineCount > 0 ? (($currentSpineIndex + 1) / $spineCount) * 100 : 0}%"
      ></div>
    </div>
    {#if chapterMinutes}
      <div class="reading-pill" title="Estimated reading time at 220 WPM">
        <span class="pill-clock">⏱</span>
        <span>~{chapterMinutes}m left</span>
      </div>
    {/if}
  </div>

  <div class="toolbar-right">
    <!-- Search Button -->
    <button class="btn btn-ghost btn-icon" on:click={openSearch} title="Search book (Cmd+F)" id="search-btn">
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="8"/>
        <path d="m21 21-4.3-4.3"/>
      </svg>
    </button>

    <!-- Bookmark Ribbon Button -->
    <button
      class="btn btn-ghost btn-icon ribbon-btn"
      class:bookmarked={isBookmarked}
      on:click={toggleBookmark}
      title={isBookmarked ? "Remove bookmark (Cmd+D)" : "Bookmark this chapter (Cmd+D)"}
      id="ribbon-btn"
    >
      <svg width="17" height="17" viewBox="0 0 24 24" fill={isBookmarked ? "var(--accent)" : "none"} stroke={isBookmarked ? "var(--accent)" : "currentColor"} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>
      </svg>
    </button>

    <div class="toolbar-divider"></div>

    <button class="btn btn-ghost btn-icon" on:click={() => changeFontSize(-0.1)} title="Decrease font size" id="font-decrease-btn">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M5 12h14"/>
      </svg>
    </button>
    <span class="font-size-label">{Math.round($settings.fontSize * 100)}%</span>
    <button class="btn btn-ghost btn-icon" on:click={() => changeFontSize(0.1)} title="Increase font size" id="font-increase-btn">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M5 12h14"/><path d="M12 5v14"/>
      </svg>
    </button>

    <div class="toolbar-divider"></div>

    <button class="btn btn-ghost btn-icon" on:click={cycleTheme} title="Change theme ({$settings.theme})" id="theme-toggle-btn">
      <span class="theme-emoji">{themeIcons[$settings.theme] || '🌙'}</span>
    </button>

    <button class="btn btn-ghost btn-icon" on:click={openSettings} title="Reader Settings" id="settings-toggle-btn">
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
    </button>
  </div>
</header>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: var(--toolbar-height);
    padding: 0 var(--space-md);
    border-bottom: 1px solid var(--border-subtle);
    background: var(--glass-bg);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    flex-shrink: 0;
    --wails-draggable: drag;
  }

  .toolbar-left, .toolbar-right {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    --wails-draggable: no-drag;
  }

  .toolbar-center {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    --wails-draggable: no-drag;
  }

  .toolbar-title {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--fg-secondary);
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-left: 4px;
  }

  .chapter-indicator {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--fg-tertiary);
    font-variant-numeric: tabular-nums;
    min-width: 45px;
    text-align: center;
  }

  .progress-bar-container {
    width: 100px;
    height: 3px;
    background: var(--bg-hover);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-bar-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width var(--duration-slow) var(--ease-in-out);
  }

  .reading-pill {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    font-size: 0.6875rem;
    font-weight: 500;
    color: var(--fg-secondary);
    letter-spacing: 0.01em;
  }

  .pill-clock {
    font-size: 0.75rem;
    opacity: 0.8;
  }

  .font-size-label {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--fg-tertiary);
    min-width: 36px;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }

  .toolbar-divider {
    width: 1px;
    height: 18px;
    background: var(--border-subtle);
    margin: 0 2px;
  }

  .theme-emoji {
    font-size: 0.9375rem;
    line-height: 1;
  }

  .ribbon-btn.bookmarked {
    color: var(--accent);
  }
</style>
