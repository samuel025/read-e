<script>
  import {
    view, currentBook, currentBookId, currentSpineIndex,
    spineCount, tocOpen, settings, settingsOpen,
    activeSidebarTab, highlights, bookmarks, searchOpen, toc,
    pdfZoom, readingStats, library
  } from '../stores/app.js';
  import {
    saveSettings, saveBookmark, deleteBookmark,
    getChapterWordCount, getLibrary
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

  $: if ($currentBookId && $currentSpineIndex !== undefined && $currentBook?.format !== 'pdf') {
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

  $: displayMinutes = $currentBook?.format === 'pdf' ? $readingStats.minutesLeft : chapterMinutes;
  $: currentBookmark = $bookmarks.find(b => b.spineIndex === $currentSpineIndex);
  $: isBookmarked = !!currentBookmark;

  async function toggleBookmark() {
    if (!$currentBookId) return;

    if (isBookmarked && currentBookmark) {
      await deleteBookmark(currentBookmark.id);
      bookmarks.update(items => items.filter(b => b.id !== currentBookmark.id));
    } else {
      let scrollY = 0;
      let title = '';
      if ($currentBook?.format === 'pdf') {
        const pdfContainer = document.querySelector('.pdf-viewport');
        scrollY = pdfContainer ? pdfContainer.scrollTop : 0;
        title = `Page ${$currentSpineIndex + 1}`;
      } else {
        const iframe = document.querySelector('.chapter-frame');
        scrollY = iframe?.contentWindow?.scrollY || iframe?.contentWindow?.document?.documentElement?.scrollTop || 0;
        title = getChapterTitle($currentSpineIndex);
      }

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

  async function goBack() {
    view.set('library');
    currentBook.set(null);
    currentBookId.set(null);
    try {
      const books = await getLibrary();
      if (books) library.set(books);
    } catch (e) {
      console.error('Failed to refresh library on back:', e);
    }
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
      {$currentBook?.format === 'pdf' ? `Page ${$currentSpineIndex + 1}` : ($currentSpineIndex + 1)} / {$spineCount}
    </span>
    <div class="progress-bar-container">
      <div
        class="progress-bar-fill"
        style="width: {$spineCount > 0 ? (($currentSpineIndex + 1) / $spineCount) * 100 : 0}%"
      ></div>
    </div>
    {#if displayMinutes}
      <div class="reading-pill" title="Estimated reading time at 220 WPM">
        <svg class="pill-clock-svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"></circle>
          <polyline points="12 6 12 12 16 14"></polyline>
        </svg>
        <span>~{displayMinutes}m left</span>
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
      title={isBookmarked ? "Remove bookmark (Cmd+D)" : "Bookmark this location (Cmd+D)"}
      id="ribbon-btn"
    >
      <svg width="17" height="17" viewBox="0 0 24 24" fill={isBookmarked ? "var(--accent)" : "none"} stroke={isBookmarked ? "var(--accent)" : "currentColor"} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>
      </svg>
    </button>

    <div class="toolbar-divider"></div>

    {#if $currentBook?.format === 'pdf'}
      <!-- PDF Zoom Controls -->
      <button
        class="btn btn-ghost btn-icon"
        on:click={() => window.dispatchEvent(new CustomEvent('pdf-zoom-out'))}
        title="Zoom Out"
        id="pdf-zoom-out-btn"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"></circle>
          <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          <line x1="8" y1="11" x2="14" y2="11"></line>
        </svg>
      </button>

      <button
        class="btn btn-ghost btn-zoom-badge"
        on:click={() => window.dispatchEvent(new CustomEvent('pdf-zoom-fit'))}
        title="Fit to width"
        id="pdf-zoom-badge"
      >
        {$pdfZoom === -1 ? 'Fit' : `${$pdfZoom}%`}
      </button>

      <button
        class="btn btn-ghost btn-icon"
        on:click={() => window.dispatchEvent(new CustomEvent('pdf-zoom-in'))}
        title="Zoom In"
        id="pdf-zoom-in-btn"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"></circle>
          <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          <line x1="11" y1="8" x2="11" y2="14"></line>
          <line x1="8" y1="11" x2="14" y2="11"></line>
        </svg>
      </button>
    {:else}
      <!-- EPUB Font Size Controls -->
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
    {/if}

    <div class="toolbar-divider"></div>

    <button class="btn btn-ghost btn-icon" on:click={cycleTheme} title="Change theme ({$settings.theme})" id="theme-toggle-btn">
      {#if $settings.theme === 'light'}
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="5"></circle>
          <line x1="12" y1="1" x2="12" y2="3"></line>
          <line x1="12" y1="21" x2="12" y2="23"></line>
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
          <line x1="1" y1="12" x2="3" y2="12"></line>
          <line x1="21" y1="12" x2="23" y2="12"></line>
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
        </svg>
      {:else if $settings.theme === 'sepia'}
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path>
          <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path>
        </svg>
      {:else}
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
        </svg>
      {/if}
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

  .pill-clock-svg {
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

  .btn-zoom-badge {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--fg-secondary);
    padding: 2px 6px;
    border-radius: 4px;
    min-width: 38px;
    text-align: center;
    font-variant-numeric: tabular-nums;
    cursor: pointer;
  }

  .btn-zoom-badge:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .toolbar-divider {
    width: 1px;
    height: 18px;
    background: var(--border-subtle);
    margin: 0 2px;
  }

  .ribbon-btn.bookmarked {
    color: var(--accent);
  }
</style>
