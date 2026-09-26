<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { library, shelves } from '../stores/app.js';
  import { getShelfBookIDs, addBookToShelf, removeBookFromShelf } from './api.js';

  export let shelf;

  const dispatch = createEventDispatcher();

  let bookIDsOnShelf = new Set();
  let searchQuery = '';
  let loading = true;

  onMount(async () => {
    try {
      const ids = await getShelfBookIDs(shelf.id);
      bookIDsOnShelf = new Set(ids || []);
    } catch (e) {
      console.error('Failed to load shelf books:', e);
    } finally {
      loading = false;
    }
  });

  function close() {
    dispatch('close');
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      close();
    }
  }

  $: filteredLibrary = ($library || []).filter(b => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    return (
      (b.title && b.title.toLowerCase().includes(q)) ||
      (b.author && b.author.toLowerCase().includes(q))
    );
  });

  async function toggleBook(book) {
    const isPresent = bookIDsOnShelf.has(book.id);
    if (isPresent) {
      bookIDsOnShelf.delete(book.id);
      bookIDsOnShelf = new Set(bookIDsOnShelf);
      shelves.update(list => list.map(s => s.id === shelf.id ? { ...s, bookCount: Math.max(0, (s.bookCount || 1) - 1) } : s));
      try {
        await removeBookFromShelf(shelf.id, book.id);
        dispatch('membership-changed', { shelfId: shelf.id, bookId: book.id, added: false });
      } catch (e) {
        console.error('Failed to remove book from shelf:', e);
        bookIDsOnShelf.add(book.id);
        bookIDsOnShelf = new Set(bookIDsOnShelf);
      }
    } else {
      bookIDsOnShelf.add(book.id);
      bookIDsOnShelf = new Set(bookIDsOnShelf);
      shelves.update(list => list.map(s => s.id === shelf.id ? { ...s, bookCount: (s.bookCount || 0) + 1 } : s));
      try {
        await addBookToShelf(shelf.id, book.id);
        dispatch('membership-changed', { shelfId: shelf.id, bookId: book.id, added: true });
      } catch (e) {
        console.error('Failed to add book to shelf:', e);
        bookIDsOnShelf.delete(book.id);
        bookIDsOnShelf = new Set(bookIDsOnShelf);
      }
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div
  class="modal-backdrop"
  on:click={close}
  role="dialog"
  aria-modal="true"
  aria-labelledby="add-books-title"
  tabindex="-1"
>
  <div class="modal-box" on:click|stopPropagation role="document">
    <div class="modal-header">
      <div class="header-info">
        <div class="shelf-tag" style="background: {shelf.color || '#818cf8'}20; color: {shelf.color || '#818cf8'};">
          <span class="dot" style="background: {shelf.color || '#818cf8'};"></span>
          {shelf.name}
        </div>
        <h2 id="add-books-title">Add Books to Shelf</h2>
      </div>
      <button class="close-btn" on:click={close} aria-label="Close modal">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 6 6 18"/><path d="m6 6 12 12"/>
        </svg>
      </button>
    </div>

    <div class="search-bar-wrap">
      <svg class="search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
      </svg>
      <input
        type="text"
        class="search-input"
        placeholder="Search books in library..."
        bind:value={searchQuery}
      />
    </div>

    <div class="modal-body">
      {#if loading}
        <div class="loading-state">
          <div class="mini-spinner"></div>
          <span>Loading books...</span>
        </div>
      {:else if ($library || []).length === 0}
        <div class="empty-state">
          <p>Your library is empty. Add books to your library first.</p>
        </div>
      {:else if filteredLibrary.length === 0}
        <div class="empty-state">
          <p>No books match "{searchQuery}"</p>
        </div>
      {:else}
        <div class="book-list">
          {#each filteredLibrary as book (book.id)}
            {@const isChecked = bookIDsOnShelf.has(book.id)}
            <button
              type="button"
              class="book-row"
              class:selected={isChecked}
              on:click={() => toggleBook(book)}
            >
              <div class="book-left">
                <div class="book-thumb">
                  {#if book.coverBase64 || book.cover_base64}
                    <img src={book.coverBase64 || book.cover_base64} alt={book.title} />
                  {:else}
                    <div class="thumb-placeholder">{book.title.slice(0, 1).toUpperCase()}</div>
                  {/if}
                </div>
                <div class="book-details">
                  <span class="book-title">{book.title}</span>
                  {#if book.author}
                    <span class="book-author">{book.author}</span>
                  {/if}
                </div>
              </div>
              <div class="checkbox-box" class:checked={isChecked}>
                {#if isChecked}
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                {/if}
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      <div class="count-indicator">
        {bookIDsOnShelf.size} {bookIDsOnShelf.size === 1 ? 'book' : 'books'} on shelf
      </div>
      <button type="button" class="btn btn-primary" on:click={close}>
        Done
      </button>
    </div>
  </div>
</div>

<style>
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
    animation: fadeIn var(--duration-fast) var(--ease-out);
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .modal-box {
    background: var(--bg-card);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 500px;
    box-shadow: var(--shadow-xl);
    display: flex;
    flex-direction: column;
    max-height: 80vh;
    overflow: hidden;
    animation: slideUp var(--duration-normal) var(--ease-out);
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(12px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  .modal-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: var(--space-md) var(--space-lg);
    border-bottom: 1px solid var(--border-subtle);
  }

  .header-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .shelf-tag {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 100px;
    width: fit-content;
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .modal-header h2 {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--fg-primary);
    margin: 0;
  }

  .close-btn {
    background: transparent;
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
  }

  .close-btn:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .search-bar-wrap {
    position: relative;
    padding: var(--space-sm) var(--space-lg);
    border-bottom: 1px solid var(--border-subtle);
  }

  .search-icon {
    position: absolute;
    left: 28px;
    top: 50%;
    transform: translateY(-50%);
    color: var(--fg-tertiary);
    pointer-events: none;
  }

  .search-input {
    width: 100%;
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    padding: 8px 12px 8px 34px;
    font-size: 0.875rem;
    font-family: var(--font-sans);
    color: var(--fg-primary);
    transition: border-color var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out);
  }

  .search-input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-subtle);
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-sm) var(--space-lg);
  }

  .book-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 4px 0;
  }

  .book-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 8px 12px;
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: background var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .book-row:hover {
    background: var(--bg-hover);
    border-color: var(--border-medium);
  }

  .book-row.selected {
    border-color: var(--accent);
    background: var(--accent-subtle);
  }

  .book-left {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    overflow: hidden;
  }

  .book-thumb {
    width: 32px;
    height: 44px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
    background: var(--bg-secondary);
    box-shadow: var(--shadow-sm);
  }

  .book-thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .thumb-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 700;
    font-size: 0.875rem;
  }

  .book-details {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .book-title {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--fg-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .book-author {
    font-size: 0.75rem;
    color: var(--fg-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .checkbox-box {
    width: 20px;
    height: 20px;
    border-radius: 4px;
    border: 1.5px solid var(--border-medium);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: background var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .checkbox-box.checked {
    background: var(--accent);
    border-color: var(--accent);
  }

  .loading-state, .empty-state {
    text-align: center;
    padding: var(--space-xl) 0;
    color: var(--fg-secondary);
    font-size: 0.875rem;
  }

  .loading-state {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-sm);
  }

  .mini-spinner {
    width: 16px;
    height: 16px;
    border: 2px solid var(--border-subtle);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-md) var(--space-lg);
    border-top: 1px solid var(--border-subtle);
  }

  .count-indicator {
    font-size: 0.8125rem;
    color: var(--fg-tertiary);
  }
</style>
