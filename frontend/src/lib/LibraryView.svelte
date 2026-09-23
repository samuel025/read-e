<script>
  import { library, libraryLoading, settings, settingsOpen } from '../stores/app.js';
  import { scanLibrary, addBook, selectFolder, selectFile, getLibrary, removeBook, getPDFData, updateBookCover, updateBookTotalCount, markBookFinished } from './api.js';
  import BookCard from './BookCard.svelte';
  import InsightsModal from './InsightsModal.svelte';

  let searchQuery = '';
  const generatingCovers = new Set();
  const coverQueue = [];
  let isProcessingCoverQueue = false;
  let showInsights = false;

  $: filteredBooks = (Array.isArray($library) ? $library : []).filter((b) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return (
      b.title.toLowerCase().includes(q) ||
      b.author.toLowerCase().includes(q)
    );
  });

  $: {
    if (Array.isArray($library)) {
      for (const b of $library) {
        const hasCover = !!(b.coverBase64 || b.cover_base64);
        const hasTotal = (b.totalCount || 0) > 0;
        if (b.format === 'pdf' && (!hasCover || !hasTotal) && !generatingCovers.has(b.id)) {
          generatingCovers.add(b.id);
          coverQueue.push({ book: b, hasCover });
        }
      }
      processCoverQueue();
    }
  }

  async function processCoverQueue() {
    if (isProcessingCoverQueue) return;
    isProcessingCoverQueue = true;
    while (coverQueue.length > 0) {
      const item = coverQueue.shift();
      try {
        await generatePDFCover(item.book, item.hasCover);
      } catch (e) {
        console.error('Error generating cover for', item.book?.title, e);
      }
    }
    isProcessingCoverQueue = false;
  }

  async function generatePDFCover(book, alreadyHasCover = false) {
    let doc = null;
    let page = null;
    let canvas = null;
    let loadingTask = null;
    try {
      const pdfjsLib = await import('pdfjs-dist');
      const pdfjsWorker = (await import('pdfjs-dist/build/pdf.worker.min.js?url')).default;
      pdfjsLib.GlobalWorkerOptions.workerSrc = pdfjsWorker;

      try {
        loadingTask = pdfjsLib.getDocument({
          url: `/pdf/${encodeURIComponent(book.id)}`,
          cMapUrl: 'https://cdn.jsdelivr.net/npm/pdfjs-dist@3.11.174/cmaps/',
          cMapPacked: true,
        });
        doc = await loadingTask.promise;
      } catch (streamErr) {
        const data = await getPDFData(book.id);
        if (!data) return;

        loadingTask = pdfjsLib.getDocument({
          data,
          cMapUrl: 'https://cdn.jsdelivr.net/npm/pdfjs-dist@3.11.174/cmaps/',
          cMapPacked: true,
        });
        doc = await loadingTask.promise;
      }
      if (doc.numPages > 0) {
        await updateBookTotalCount(book.id, doc.numPages);
      }

      let base64 = book.coverBase64 || book.cover_base64 || '';
      if (!alreadyHasCover) {
        page = await doc.getPage(1);

        const viewport = page.getViewport({ scale: 1.0 });
        const scale = 400 / viewport.width;
        const scaledViewport = page.getViewport({ scale });

        canvas = document.createElement('canvas');
        canvas.width = scaledViewport.width;
        canvas.height = scaledViewport.height;
        const ctx = canvas.getContext('2d');

        await page.render({ canvasContext: ctx, viewport: scaledViewport }).promise;

        base64 = canvas.toDataURL('image/jpeg', 0.8);
        await updateBookCover(book.id, base64);
      }

      library.update(lib => {
        return lib.map(l => {
          if (l.id === book.id) {
            const totalCount = doc.numPages || l.totalCount || 0;
            const progress = l.finished ? 100 : (totalCount > 0 && l.hasProgress ? Math.round(((l.spineIndex || 0) + 1) / totalCount * 100) : (l.hasProgress ? 5 : 0));
            return {
              ...l,
              coverBase64: base64,
              cover_base64: base64,
              totalCount,
              progress,
            };
          }
          return l;
        });
      });
    } catch (e) {
      console.error('Failed to process PDF cover / count for', book.title, e);
    } finally {
      if (page) {
        try { page.cleanup(); } catch (_) {}
      }
      if (canvas) {
        canvas.width = 0;
        canvas.height = 0;
      }
      if (doc) {
        try { doc.destroy(); } catch (_) {}
      }
      if (loadingTask) {
        try { loadingTask.destroy(); } catch (_) {}
      }
      generatingCovers.delete(book.id);
    }
  }

  async function handleAddFolder() {
    const dir = await selectFolder();
    if (!dir) return;
    libraryLoading.set(true);
    try {
      const books = await scanLibrary(dir);
      if (books && Array.isArray(books)) library.set(books);
    } finally {
      libraryLoading.set(false);
    }
  }

  async function handleAddFile() {
    const file = await selectFile();
    if (!file) return;
    libraryLoading.set(true);
    try {
      const books = await addBook(file);
      if (books && Array.isArray(books)) library.set(books);
    } finally {
      libraryLoading.set(false);
    }
  }

  async function handleRemoveBook(e) {
    const bookId = e.detail;
    await removeBook(bookId);
    const books = await getLibrary();
    if (books && Array.isArray(books)) library.set(books);
  }

  async function handleToggleFinished(e) {
    const { bookId, finished } = e.detail;
    await markBookFinished(bookId, finished);
    library.update(lib => {
      return lib.map(b => {
        if (b.id === bookId) {
          const progress = finished ? 100 : (b.totalCount > 0 && b.hasProgress ? Math.round(((b.spineIndex || 0) + 1) / b.totalCount * 100) : (b.hasProgress ? 5 : 0));
          return {
            ...b,
            finished,
            progress,
          };
        }
        return b;
      });
    });
  }
</script>

<div class="library-view">
  <!-- Header / Toolbar -->
  <header class="library-header">
    <div class="header-left">
      <h1 class="app-title">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
        </svg>
        Library
      </h1>
      <span class="book-count">{$library.length} {$library.length === 1 ? 'book' : 'books'}</span>
    </div>
    <div class="header-actions">
      <button class="btn btn-outline" on:click={() => showInsights = true}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 20V10M18 20V4M6 20v-4" />
        </svg>
        Insights
      </button>
      <div class="search-wrapper">
        <svg class="search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
        </svg>
        <input
          type="text"
          class="search-input"
          placeholder="Search books..."
          bind:value={searchQuery}
        />
      </div>
      <button class="btn btn-secondary" on:click={handleAddFile} id="add-file-btn">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M5 12h14"/><path d="M12 5v14"/>
        </svg>
        Add File
      </button>
      <button class="btn btn-primary" on:click={handleAddFolder} id="add-folder-btn">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"/>
        </svg>
        Add Folder
      </button>
      <button
        class="btn btn-icon btn-ghost"
        on:click={() => settingsOpen.set(true)}
        id="settings-btn"
        title="Settings"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
          <circle cx="12" cy="12" r="3"/>
        </svg>
      </button>
    </div>
  </header>

  <!-- Content -->
  <main class="library-content">
    {#if $libraryLoading}
      <div class="loading-state">
        <div class="spinner"></div>
        <p>Scanning for books...</p>
      </div>
    {:else if $library.length === 0}
      <div class="empty-state">
        <div class="empty-icon">
          <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.4">
            <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
            <path d="M12 6v7"/><path d="m15 9-3-3-3 3"/>
          </svg>
        </div>
        <h2>Your library is empty</h2>
        <p>Add EPUB files or scan a folder to get started</p>
        <div class="empty-actions">
          <button class="btn btn-primary" on:click={handleAddFolder}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"/>
            </svg>
            Scan Folder
          </button>
          <button class="btn btn-secondary" on:click={handleAddFile}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M5 12h14"/><path d="M12 5v14"/>
            </svg>
            Add File
          </button>
        </div>
      </div>
    {:else if filteredBooks.length === 0}
      <div class="empty-state">
        <p>No books match "{searchQuery}"</p>
      </div>
    {:else}
      <div class="book-grid">
        {#each filteredBooks as book (book.id)}
          <div class="stagger-item">
            <BookCard
              {book}
              on:remove={handleRemoveBook}
              on:toggle-finished={handleToggleFinished}
            />
          </div>
        {/each}
      </div>
    {/if}
  </main>
</div>

{#if showInsights}
  <InsightsModal on:close={() => showInsights = false} />
{/if}

<style>
  @keyframes viewFadeIn {
    from {
      opacity: 0;
      transform: scale(0.995);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .library-view {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-primary);
    animation: viewFadeIn 160ms var(--ease-out) forwards;
    will-change: opacity, transform;
  }

  .library-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-md) var(--space-xl);
    border-bottom: 1px solid var(--border-subtle);
    background: var(--glass-bg);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    /* Wails: drag region for moving the window */
    --wails-draggable: drag;
    height: var(--toolbar-height);
    flex-shrink: 0;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    --wails-draggable: no-drag;
  }

  .app-title {
    font-size: 1.125rem;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    color: var(--fg-primary);
  }

  .book-count {
    font-size: 0.8125rem;
    color: var(--fg-tertiary);
    background: var(--bg-hover);
    padding: 2px 10px;
    border-radius: 100px;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    --wails-draggable: no-drag;
  }

  .search-wrapper {
    position: relative;
    display: flex;
    align-items: center;
  }

  .search-icon {
    position: absolute;
    left: 10px;
    color: var(--fg-tertiary);
    pointer-events: none;
  }

  .search-input {
    background: var(--bg-hover);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    padding: 6px 12px 6px 32px;
    font-size: 0.8125rem;
    font-family: var(--font-sans);
    color: var(--fg-primary);
    width: 220px;
    transition: border-color var(--duration-fast) var(--ease-out),
                background var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out);
  }

  .search-input:focus {
    outline: none;
    border-color: var(--accent);
    background: var(--bg-secondary);
    box-shadow: 0 0 0 2px var(--accent-subtle);
  }

  .search-input::placeholder {
    color: var(--fg-tertiary);
  }

  /* Content area */
  .library-content {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-xl);
  }

  .book-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: var(--space-lg);
  }

  /* Empty state */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: var(--space-md);
    text-align: center;
    color: var(--fg-secondary);
  }

  .empty-state h2 {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--fg-primary);
  }

  .empty-state p {
    font-size: 0.9375rem;
    max-width: 360px;
  }

  .empty-icon {
    margin-bottom: var(--space-md);
  }

  .empty-actions {
    display: flex;
    gap: var(--space-sm);
    margin-top: var(--space-md);
  }

  /* Loading */
  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: var(--space-md);
    color: var(--fg-secondary);
  }

  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
