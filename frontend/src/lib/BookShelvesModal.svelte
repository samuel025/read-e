<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { shelves } from '../stores/app.js';
  import { getBookShelfIDs, addBookToShelf, removeBookFromShelf } from './api.js';

  export let book;

  const dispatch = createEventDispatcher();

  let assignedShelfIDs = new Set();
  let loading = true;
  let showCreateModal = false;

  onMount(async () => {
    try {
      const ids = await getBookShelfIDs(book.id);
      assignedShelfIDs = new Set(ids || []);
    } catch (e) {
      console.error('Failed to load book shelves:', e);
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

  async function toggleShelf(shelf) {
    const isAssigned = assignedShelfIDs.has(shelf.id);
    // Optimistic update
    if (isAssigned) {
      assignedShelfIDs.delete(shelf.id);
      assignedShelfIDs = new Set(assignedShelfIDs);
      shelves.update(list => list.map(s => s.id === shelf.id ? { ...s, bookCount: Math.max(0, (s.bookCount || 1) - 1) } : s));
      try {
        await removeBookFromShelf(shelf.id, book.id);
        dispatch('shelf-changed', { bookId: book.id, shelfId: shelf.id, added: false });
      } catch (err) {
        console.error('Failed to remove from shelf:', err);
        assignedShelfIDs.add(shelf.id);
        assignedShelfIDs = new Set(assignedShelfIDs);
      }
    } else {
      assignedShelfIDs.add(shelf.id);
      assignedShelfIDs = new Set(assignedShelfIDs);
      shelves.update(list => list.map(s => s.id === shelf.id ? { ...s, bookCount: (s.bookCount || 0) + 1 } : s));
      try {
        await addBookToShelf(shelf.id, book.id);
        dispatch('shelf-changed', { bookId: book.id, shelfId: shelf.id, added: true });
      } catch (err) {
        console.error('Failed to add to shelf:', err);
        assignedShelfIDs.delete(shelf.id);
        assignedShelfIDs = new Set(assignedShelfIDs);
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
  aria-labelledby="book-shelves-title"
  tabindex="-1"
>
  <div class="modal-box" on:click|stopPropagation role="document">
    <div class="modal-header">
      <div class="book-summary">
        <div class="book-mini-cover">
          {#if book.coverBase64 || book.cover_base64}
            <img src={book.coverBase64 || book.cover_base64} alt={book.title} />
          {:else}
            <div class="mini-placeholder">
              {book.title.slice(0, 1).toUpperCase()}
            </div>
          {/if}
        </div>
        <div class="book-text">
          <h2 id="book-shelves-title">{book.title}</h2>
          {#if book.author}
            <span class="book-author">{book.author}</span>
          {/if}
        </div>
      </div>
      <button class="close-btn" on:click={close} aria-label="Close modal">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 6 6 18"/><path d="m6 6 12 12"/>
        </svg>
      </button>
    </div>

    <div class="modal-body">
      <div class="section-heading">
        <span>Organize into Shelves</span>
        <button class="btn-new-shelf" on:click={() => dispatch('open-new-shelf')}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          New Shelf
        </button>
      </div>

      {#if loading}
        <div class="loading-state">
          <div class="mini-spinner"></div>
          <span>Loading shelves...</span>
        </div>
      {:else if $shelves.length === 0}
        <div class="empty-shelves">
          <p>No shelves created yet.</p>
          <button class="btn btn-secondary btn-sm" on:click={() => dispatch('open-new-shelf')}>
            Create your first shelf
          </button>
        </div>
      {:else}
        <div class="shelves-list">
          {#each $shelves as shelf (shelf.id)}
            {@const isChecked = assignedShelfIDs.has(shelf.id)}
            <button
              type="button"
              class="shelf-row-item"
              class:is-active={isChecked}
              on:click={() => toggleShelf(shelf)}
            >
              <div class="shelf-row-left">
                <span class="shelf-color-badge" style="background-color: {shelf.color || '#818cf8'};"></span>
                <span class="shelf-row-name">{shelf.name}</span>
                {#if shelf.bookCount !== undefined}
                  <span class="shelf-row-count">{shelf.bookCount}</span>
                {/if}
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
    max-width: 440px;
    box-shadow: var(--shadow-xl);
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

  .book-summary {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    max-width: 340px;
  }

  .book-mini-cover {
    width: 38px;
    height: 52px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
    background: var(--bg-secondary);
    box-shadow: var(--shadow-sm);
  }

  .book-mini-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .mini-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 700;
    font-size: 1.125rem;
  }

  .book-text {
    overflow: hidden;
  }

  .book-text h2 {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--fg-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin: 0;
  }

  .book-author {
    font-size: 0.8125rem;
    color: var(--fg-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    margin-top: 2px;
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

  .modal-body {
    padding: var(--space-md) var(--space-lg);
    max-height: 360px;
    overflow-y: auto;
  }

  .section-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-sm);
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--fg-tertiary);
  }

  .btn-new-shelf {
    background: none;
    border: none;
    color: var(--accent);
    font-size: 0.75rem;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    transition: background var(--duration-fast) var(--ease-out);
  }

  .btn-new-shelf:hover {
    background: var(--accent-subtle);
  }

  .shelves-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .shelf-row-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 10px 12px;
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: background var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .shelf-row-item:hover {
    background: var(--bg-hover);
    border-color: var(--border-medium);
  }

  .shelf-row-item.is-active {
    border-color: var(--accent);
    background: var(--accent-subtle);
  }

  .shelf-row-left {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .shelf-color-badge {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .shelf-row-name {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--fg-primary);
  }

  .shelf-row-count {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
    background: var(--bg-hover);
    padding: 2px 6px;
    border-radius: 100px;
  }

  .checkbox-box {
    width: 20px;
    height: 20px;
    border-radius: 4px;
    border: 1.5px solid var(--border-medium);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .checkbox-box.checked {
    background: var(--accent);
    border-color: var(--accent);
  }

  .loading-state {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-sm);
    padding: var(--space-xl) 0;
    color: var(--fg-tertiary);
    font-size: 0.875rem;
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

  .empty-shelves {
    text-align: center;
    padding: var(--space-lg) 0;
    color: var(--fg-secondary);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-sm);
    font-size: 0.875rem;
  }

  .btn-sm {
    padding: 4px 10px;
    font-size: 0.8125rem;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: var(--space-md) var(--space-lg);
    border-top: 1px solid var(--border-subtle);
  }
</style>
