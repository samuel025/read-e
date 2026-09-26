<script>
  import { onMount } from 'svelte';
  import { library, libraryLoading, settings, settingsOpen, shelves, activeShelfId } from '../stores/app.js';
  import {
    scanLibrary, addBook, selectFolder, selectFile, getLibrary, removeBook,
    getPDFData, updateBookCover, updateBookTotalCount, markBookFinished,
    getShelves, deleteShelf, getShelfBookIDs, removeBookFromShelf
  } from './api.js';
  import BookCard from './BookCard.svelte';
  import InsightsModal from './InsightsModal.svelte';
  import ShelfModal from './ShelfModal.svelte';
  import BookShelvesModal from './BookShelvesModal.svelte';
  import AddBooksToShelfModal from './AddBooksToShelfModal.svelte';

  let searchQuery = '';
  const generatingCovers = new Set();
  const coverQueue = [];
  let isProcessingCoverQueue = false;

  // Modals & UI state
  let showInsights = false;
  let showShelfModal = false;
  let editingShelf = null;
  let managingBook = null;
  let showAddBooksModal = false;
  let deleteConfirmShelf = null;
  let sidebarCollapsed = false;

  // Set of book IDs currently on the active custom shelf
  let activeShelfBookIDs = new Set();
  let loadingShelfBooks = false;

  onMount(async () => {
    await reloadShelves();
  });

  async function reloadShelves() {
    try {
      const data = await getShelves();
      if (data && Array.isArray(data)) {
        shelves.set(data);
      }
    } catch (e) {
      console.error('Failed to load shelves:', e);
    }
  }

  // When activeShelfId changes, load its book IDs if it's a custom shelf
  $: if ($activeShelfId && $activeShelfId !== 'all' && $activeShelfId !== 'reading' && $activeShelfId !== 'finished') {
    loadShelfBooks($activeShelfId);
  } else {
    activeShelfBookIDs = new Set();
  }

  async function loadShelfBooks(shelfId) {
    loadingShelfBooks = true;
    try {
      const ids = await getShelfBookIDs(shelfId);
      activeShelfBookIDs = new Set(ids || []);
    } catch (e) {
      console.error('Failed to load shelf book IDs:', e);
    } finally {
      loadingShelfBooks = false;
    }
  }

  // Active shelf object (if custom)
  $: currentShelf = ($shelves || []).find(s => s.id === $activeShelfId) || null;
  $: isCustomShelf = !!currentShelf;

  // Counts for standard views
  $: totalBooksCount = (Array.isArray($library) ? $library : []).length;
  $: readingBooksCount = (Array.isArray($library) ? $library : []).filter(b => b.hasProgress && !b.finished).length;
  $: finishedBooksCount = (Array.isArray($library) ? $library : []).filter(b => b.finished).length;

  // Filtered books based on active shelf and search query
  $: filteredBooks = (Array.isArray($library) ? $library : []).filter((b) => {
    // 1. Shelf filtering
    if ($activeShelfId === 'reading') {
      if (!b.hasProgress || b.finished) return false;
    } else if ($activeShelfId === 'finished') {
      if (!b.finished) return false;
    } else if (isCustomShelf) {
      if (!activeShelfBookIDs.has(b.id)) return false;
    }

    // 2. Search query filtering
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    return (
      (b.title && b.title.toLowerCase().includes(q)) ||
      (b.author && b.author.toLowerCase().includes(q))
    );
  });

  // Background PDF cover generation
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
    await reloadShelves();
    if (isCustomShelf) {
      activeShelfBookIDs.delete(bookId);
      activeShelfBookIDs = new Set(activeShelfBookIDs);
    }
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

  // Shelf operations
  function handleSelectShelf(id) {
    activeShelfId.set(id);
  }

  function handleOpenNewShelf() {
    editingShelf = null;
    showShelfModal = true;
  }

  function handleEditShelf(shelf, e) {
    if (e) e.stopPropagation();
    editingShelf = shelf;
    showShelfModal = true;
  }

  function handleDeleteShelfClick(shelf, e) {
    if (e) e.stopPropagation();
    deleteConfirmShelf = shelf;
  }

  async function handleConfirmDeleteShelf() {
    if (!deleteConfirmShelf) return;
    const shelfId = deleteConfirmShelf.id;
    try {
      await deleteShelf(shelfId);
      shelves.update(list => list.filter(s => s.id !== shelfId));
      if ($activeShelfId === shelfId) {
        activeShelfId.set('all');
      }
    } catch (err) {
      console.error('Failed to delete shelf:', err);
    } finally {
      deleteConfirmShelf = null;
    }
  }

  async function handleRemoveFromShelf(e) {
    const bookId = e.detail;
    if (!currentShelf) return;
    activeShelfBookIDs.delete(bookId);
    activeShelfBookIDs = new Set(activeShelfBookIDs);
    shelves.update(list => list.map(s => s.id === currentShelf.id ? { ...s, bookCount: Math.max(0, (s.bookCount || 1) - 1) } : s));
    try {
      await removeBookFromShelf(currentShelf.id, bookId);
    } catch (err) {
      console.error('Failed to remove book from shelf:', err);
      activeShelfBookIDs.add(bookId);
      activeShelfBookIDs = new Set(activeShelfBookIDs);
    }
  }

  function handleManageShelves(e) {
    managingBook = e.detail;
  }

  async function handleShelfMembershipChanged(e) {
    const { shelfId, bookId, added } = e.detail;
    if (currentShelf && currentShelf.id === shelfId) {
      if (added) {
        activeShelfBookIDs.add(bookId);
      } else {
        activeShelfBookIDs.delete(bookId);
      }
      activeShelfBookIDs = new Set(activeShelfBookIDs);
    }
    await reloadShelves();
  }
</script>

<div class="library-layout">
  <!-- Left Navigation Sidebar -->
  <aside class="library-sidebar" class:collapsed={sidebarCollapsed}>
    <!-- Sidebar Header -->
    <div class="sidebar-header">
      <div class="brand">
        <svg class="brand-icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
          <path d="M9 10h6"/>
        </svg>
        <span class="brand-title">Read-e</span>
      </div>
      <button
        class="collapse-toggle-btn"
        on:click={() => sidebarCollapsed = !sidebarCollapsed}
        title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
          <line x1="9" y1="3" x2="9" y2="21"/>
        </svg>
      </button>
    </div>

    <!-- Core Views -->
    <div class="sidebar-section">
      <span class="section-title">Library</span>
      <nav class="nav-list">
        <button
          class="nav-item"
          class:active={$activeShelfId === 'all'}
          on:click={() => handleSelectShelf('all')}
        >
          <div class="nav-left">
            <svg class="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
            </svg>
            <span>All Books</span>
          </div>
          <span class="nav-badge">{totalBooksCount}</span>
        </button>

        <button
          class="nav-item"
          class:active={$activeShelfId === 'reading'}
          on:click={() => handleSelectShelf('reading')}
        >
          <div class="nav-left">
            <svg class="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/>
              <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>
            </svg>
            <span>Currently Reading</span>
          </div>
          <span class="nav-badge reading">{readingBooksCount}</span>
        </button>

        <button
          class="nav-item"
          class:active={$activeShelfId === 'finished'}
          on:click={() => handleSelectShelf('finished')}
        >
          <div class="nav-left">
            <svg class="nav-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
              <polyline points="22 4 12 14.01 9 11.01"/>
            </svg>
            <span>Finished</span>
          </div>
          <span class="nav-badge finished">{finishedBooksCount}</span>
        </button>
      </nav>
    </div>

    <!-- Shelves Section -->
    <div class="sidebar-section shelves-section">
      <div class="section-header-row">
        <span class="section-title">Shelves</span>
        <button
          class="btn-add-shelf"
          on:click={handleOpenNewShelf}
          title="Create New Shelf"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
        </button>
      </div>

      <nav class="nav-list">
        {#if ($shelves || []).length === 0}
          <div class="empty-sidebar-shelves">
            <span>No custom shelves yet</span>
            <button class="btn-create-prompt" on:click={handleOpenNewShelf}>
              + Create a shelf
            </button>
          </div>
        {:else}
          {#each $shelves as shelf (shelf.id)}
            <div
              class="shelf-nav-row"
              class:active={$activeShelfId === shelf.id}
              on:click={() => handleSelectShelf(shelf.id)}
              role="button"
              tabindex="0"
              on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleSelectShelf(shelf.id); }}
            >
              <div class="nav-left">
                <span class="shelf-color-indicator" style="background-color: {shelf.color || '#818cf8'};"></span>
                <span class="shelf-name">{shelf.name}</span>
              </div>
              <div class="shelf-nav-right">
                <span class="nav-badge">{shelf.bookCount || 0}</span>
                <div class="shelf-hover-actions">
                  <button
                    class="shelf-action-btn edit-shelf-btn"
                    title="Edit shelf"
                    on:click={(e) => handleEditShelf(shelf, e)}
                  >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z"/>
                    </svg>
                  </button>
                  <button
                    class="shelf-action-btn delete-shelf-btn"
                    title="Delete shelf"
                    on:click={(e) => handleDeleteShelfClick(shelf, e)}
                  >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <line x1="18" y1="6" x2="6" y2="18"/>
                      <line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          {/each}
        {/if}
      </nav>
    </div>

    <!-- Sidebar Footer -->
    <div class="sidebar-footer">
      <button class="footer-btn" on:click={() => showInsights = true}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 20V10M18 20V4M6 20v-4" />
        </svg>
        <span>Reading Insights</span>
      </button>
    </div>
  </aside>

  <!-- Main Library View -->
  <div class="library-main">
    <!-- Header / Toolbar -->
    <header class="library-header">
      <div class="header-left">
        {#if sidebarCollapsed}
          <button
            class="btn btn-icon btn-ghost toggle-sidebar-main-btn"
            on:click={() => sidebarCollapsed = false}
            title="Show sidebar"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
              <line x1="9" y1="3" x2="9" y2="21"/>
            </svg>
          </button>
        {/if}

        <div class="title-wrap">
          {#if isCustomShelf}
            <div class="shelf-header-tag">
              <span class="shelf-color-indicator lg" style="background-color: {currentShelf.color || '#818cf8'};"></span>
              <h1 class="view-title">{currentShelf.name}</h1>
              {#if currentShelf.description}
                <span class="shelf-desc-hint" title={currentShelf.description}>— {currentShelf.description}</span>
              {/if}
            </div>
          {:else if $activeShelfId === 'reading'}
            <h1 class="view-title">Currently Reading</h1>
          {:else if $activeShelfId === 'finished'}
            <h1 class="view-title">Finished Books</h1>
          {:else}
            <h1 class="view-title">All Books</h1>
          {/if}
          <span class="book-count">{filteredBooks.length} {filteredBooks.length === 1 ? 'book' : 'books'}</span>
        </div>
      </div>

      <div class="header-actions">
        {#if isCustomShelf}
          <button class="btn btn-accent-subtle" on:click={() => showAddBooksModal = true}>
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="5" x2="12" y2="19"/>
              <line x1="5" y1="12" x2="19" y2="12"/>
            </svg>
            Add Books to Shelf
          </button>
          <button class="btn btn-icon btn-ghost" on:click={(e) => handleEditShelf(currentShelf, e)} title="Edit shelf">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z"/>
            </svg>
          </button>
        {/if}

        <div class="search-wrapper">
          <svg class="search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
          </svg>
          <input
            type="text"
            class="search-input"
            placeholder={isCustomShelf ? `Search in ${currentShelf.name}...` : "Search books..."}
            bind:value={searchQuery}
          />
        </div>

        <button class="btn btn-secondary" on:click={handleAddFile} id="add-file-btn">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M5 12h14"/><path d="M12 5v14"/>
          </svg>
          Add File
        </button>

        <button class="btn btn-primary" on:click={handleAddFolder} id="add-folder-btn">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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

    <!-- Books Grid Content -->
    <main class="library-content">
      {#if $libraryLoading || loadingShelfBooks}
        <div class="loading-state">
          <div class="spinner"></div>
          <p>{$libraryLoading ? 'Scanning for books...' : 'Loading shelf...'}</p>
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
          <p>Add EPUB or PDF files or scan a folder to get started</p>
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
      {:else if isCustomShelf && activeShelfBookIDs.size === 0}
        <div class="empty-state">
          <div class="shelf-empty-icon" style="background: {currentShelf.color || '#818cf8'}15; color: {currentShelf.color || '#818cf8'};">
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
              <line x1="9" y1="10" x2="15" y2="10"/>
            </svg>
          </div>
          <h2>This shelf is empty</h2>
          <p>Organize books into "{currentShelf.name}" to easily access them here</p>
          <button class="btn btn-primary" on:click={() => showAddBooksModal = true}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="5" x2="12" y2="19"/>
              <line x1="5" y1="12" x2="19" y2="12"/>
            </svg>
            Add Books to Shelf
          </button>
        </div>
      {:else if filteredBooks.length === 0}
        <div class="empty-state">
          {#if searchQuery}
            <p>No books match "{searchQuery}"</p>
          {:else if $activeShelfId === 'reading'}
            <p>No books currently in progress. Start reading any book to track your progress here!</p>
          {:else if $activeShelfId === 'finished'}
            <p>No finished books yet. Read through to the end of any book to see it here!</p>
          {/if}
        </div>
      {:else}
        <div class="book-grid">
          {#each filteredBooks as book (book.id)}
            <div class="stagger-item">
              <BookCard
                {book}
                {isCustomShelf}
                on:remove={handleRemoveBook}
                on:toggle-finished={handleToggleFinished}
                on:manage-shelves={handleManageShelves}
                on:remove-from-shelf={handleRemoveFromShelf}
              />
            </div>
          {/each}
        </div>
      {/if}
    </main>
  </div>
</div>

<!-- Shelf Create/Edit Modal -->
{#if showShelfModal}
  <ShelfModal
    shelf={editingShelf}
    on:close={() => { showShelfModal = false; editingShelf = null; }}
    on:created={(e) => {
      activeShelfId.set(e.detail.id);
    }}
  />
{/if}

<!-- Add Books To Shelf Modal -->
{#if showAddBooksModal && currentShelf}
  <AddBooksToShelfModal
    shelf={currentShelf}
    on:close={() => showAddBooksModal = false}
    on:membership-changed={handleShelfMembershipChanged}
  />
{/if}

<!-- Book Shelves Organization Modal -->
{#if managingBook}
  <BookShelvesModal
    book={managingBook}
    on:close={() => managingBook = null}
    on:open-new-shelf={() => {
      managingBook = null;
      handleOpenNewShelf();
    }}
    on:shelf-changed={handleShelfMembershipChanged}
  />
{/if}

<!-- Delete Shelf Confirmation Dialog -->
{#if deleteConfirmShelf}
  <div class="modal-backdrop" on:click={() => deleteConfirmShelf = null} role="dialog" aria-modal="true" tabindex="-1">
    <div class="confirm-dialog" on:click|stopPropagation role="document">
      <h3>Delete Shelf?</h3>
      <p>Are you sure you want to delete <strong>"{deleteConfirmShelf.name}"</strong>? The books on this shelf will remain in your library.</p>
      <div class="confirm-actions">
        <button class="btn btn-secondary" on:click={() => deleteConfirmShelf = null}>
          Cancel
        </button>
        <button class="btn btn-danger" on:click={handleConfirmDeleteShelf}>
          Delete Shelf
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Insights Modal -->
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

  .library-layout {
    height: 100vh;
    display: flex;
    background: var(--bg-primary);
    animation: viewFadeIn 160ms var(--ease-out) forwards;
    will-change: opacity, transform;
    overflow: hidden;
  }

  /* ---- Left Navigation Sidebar ---- */
  .library-sidebar {
    width: 240px;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    transition: width var(--duration-normal) var(--ease-drawer),
                margin-left var(--duration-normal) var(--ease-drawer);
    overflow: hidden;
    user-select: none;
    -webkit-user-select: none;
    z-index: 10;
  }

  .library-sidebar.collapsed {
    width: 0;
    margin-left: -1px;
    border-right: none;
  }

  .sidebar-header {
    height: var(--toolbar-height);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--space-md);
    border-bottom: 1px solid var(--border-subtle);
    --wails-draggable: drag;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    color: var(--fg-primary);
    --wails-draggable: no-drag;
  }

  .brand-icon {
    color: var(--accent);
  }

  .brand-title {
    font-size: 1rem;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .collapse-toggle-btn {
    background: none;
    border: none;
    color: var(--fg-tertiary);
    cursor: pointer;
    padding: 6px;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
    --wails-draggable: no-drag;
  }

  .collapse-toggle-btn:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .sidebar-section {
    padding: var(--space-md) var(--space-sm) var(--space-sm) var(--space-sm);
  }

  .shelves-section {
    flex: 1;
    overflow-y: auto;
    border-top: 1px solid var(--border-subtle);
    margin-top: var(--space-xs);
  }

  .section-header-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--space-sm) var(--space-xs) var(--space-sm);
  }

  .section-title {
    font-size: 0.6875rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--fg-tertiary);
    padding: 0 var(--space-sm);
  }

  .btn-add-shelf {
    background: none;
    border: none;
    color: var(--fg-tertiary);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
  }

  .btn-add-shelf:hover {
    background: var(--bg-hover);
    color: var(--accent);
  }

  .nav-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-top: 4px;
  }

  .nav-item, .shelf-nav-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 7px 10px;
    border-radius: var(--radius-sm);
    border: none;
    background: transparent;
    color: var(--fg-secondary);
    font-family: var(--font-sans);
    font-size: 0.84375rem;
    font-weight: 500;
    cursor: pointer;
    text-align: left;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
    position: relative;
  }

  .nav-item:hover, .shelf-nav-row:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .nav-item.active, .shelf-nav-row.active {
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 600;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 10px;
    overflow: hidden;
  }

  .nav-icon {
    flex-shrink: 0;
    color: var(--fg-tertiary);
  }

  .nav-item.active .nav-icon {
    color: var(--accent);
  }

  .shelf-color-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .shelf-color-indicator.lg {
    width: 12px;
    height: 12px;
  }

  .shelf-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .nav-badge {
    font-size: 0.6875rem;
    color: var(--fg-tertiary);
    background: var(--bg-hover);
    padding: 1px 7px;
    border-radius: 100px;
    font-weight: 500;
    flex-shrink: 0;
  }

  .nav-item.active .nav-badge, .shelf-nav-row.active .nav-badge {
    background: rgba(129, 140, 248, 0.2);
    color: var(--accent);
  }

  .shelf-nav-right {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .shelf-hover-actions {
    display: none;
    align-items: center;
    gap: 2px;
  }

  .shelf-nav-row:hover .shelf-hover-actions {
    display: flex;
  }

  .shelf-nav-row:hover .nav-badge {
    display: none;
  }

  .shelf-action-btn {
    background: transparent;
    border: none;
    color: var(--fg-tertiary);
    cursor: pointer;
    padding: 3px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
  }

  .shelf-action-btn:hover {
    background: var(--bg-active);
    color: var(--fg-primary);
  }

  .delete-shelf-btn:hover {
    color: #ef4444;
  }

  .empty-sidebar-shelves {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: var(--space-sm) var(--space-md);
    font-size: 0.75rem;
    color: var(--fg-tertiary);
  }

  .btn-create-prompt {
    background: none;
    border: none;
    color: var(--accent);
    cursor: pointer;
    text-align: left;
    font-size: 0.75rem;
    font-weight: 600;
    padding: 2px 0;
  }

  .btn-create-prompt:hover {
    text-decoration: underline;
  }

  .sidebar-footer {
    padding: var(--space-sm);
    border-top: 1px solid var(--border-subtle);
  }

  .footer-btn {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    width: 100%;
    padding: 8px 10px;
    border-radius: var(--radius-sm);
    border: none;
    background: transparent;
    color: var(--fg-secondary);
    font-family: var(--font-sans);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
  }

  .footer-btn:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  /* ---- Main Library Content Area ---- */
  .library-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    overflow: hidden;
  }

  .library-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--space-xl);
    border-bottom: 1px solid var(--border-subtle);
    background: var(--glass-bg);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    --wails-draggable: drag;
    height: var(--toolbar-height);
    flex-shrink: 0;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    min-width: 0;
    --wails-draggable: no-drag;
  }

  .toggle-sidebar-main-btn {
    flex-shrink: 0;
  }

  .title-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    min-width: 0;
  }

  .shelf-header-tag {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    min-width: 0;
  }

  .shelf-desc-hint {
    font-size: 0.8125rem;
    color: var(--fg-tertiary);
    font-weight: 400;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 240px;
  }

  .view-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--fg-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .book-count {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
    background: var(--bg-hover);
    padding: 2px 8px;
    border-radius: 100px;
    flex-shrink: 0;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    --wails-draggable: no-drag;
  }

  .btn-accent-subtle {
    background: var(--accent-subtle);
    color: var(--accent);
    border: 1px solid rgba(129, 140, 248, 0.25);
    font-size: 0.8125rem;
    font-weight: 600;
    padding: 6px 12px;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out),
                transform var(--duration-fast) var(--ease-out);
  }

  .btn-accent-subtle:hover {
    background: rgba(129, 140, 248, 0.2);
  }

  .btn-accent-subtle:active {
    transform: scale(0.97);
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
    width: 200px;
    transition: border-color var(--duration-fast) var(--ease-out),
                background var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out),
                width var(--duration-fast) var(--ease-out);
  }

  .search-input:focus {
    outline: none;
    border-color: var(--accent);
    background: var(--bg-secondary);
    box-shadow: 0 0 0 2px var(--accent-subtle);
    width: 240px;
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
    grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
    gap: var(--space-lg);
  }

  /* Empty state */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 300px;
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

  .shelf-empty-icon {
    width: 72px;
    height: 72px;
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: var(--space-xs);
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

  /* Confirm dialog */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: var(--space-md);
  }

  .confirm-dialog {
    background: var(--bg-card);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-md);
    padding: var(--space-lg);
    width: 100%;
    max-width: 400px;
    box-shadow: var(--shadow-xl);
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  .confirm-dialog h3 {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--fg-primary);
  }

  .confirm-dialog p {
    font-size: 0.875rem;
    color: var(--fg-secondary);
    line-height: 1.5;
  }

  .confirm-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-sm);
    margin-top: var(--space-xs);
  }

  .btn-danger {
    background: #ef4444;
    color: #fff;
    border: none;
    border-radius: var(--radius-sm);
    padding: var(--space-sm) var(--space-md);
    font-family: var(--font-sans);
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out),
                transform var(--duration-fast) var(--ease-out);
  }

  .btn-danger:hover {
    background: #dc2626;
  }

  .btn-danger:active {
    transform: scale(0.97);
  }
</style>
