<script>
  import { createEventDispatcher } from 'svelte';
  import { createShelf, updateShelf } from './api.js';
  import { shelves } from '../stores/app.js';

  export let shelf = null; // if set, editing mode

  const dispatch = createEventDispatcher();

  let name = shelf ? shelf.name : '';
  let description = shelf ? shelf.description : '';
  let color = shelf ? shelf.color : '#818cf8';
  let isSaving = false;
  let errorMsg = '';

  const PRESET_COLORS = [
    { label: 'Indigo', hex: '#818cf8' },
    { label: 'Emerald', hex: '#10b981' },
    { label: 'Amber', hex: '#f59e0b' },
    { label: 'Rose', hex: '#f43f5e' },
    { label: 'Sky', hex: '#0ea5e9' },
    { label: 'Violet', hex: '#a855f7' },
    { label: 'Teal', hex: '#14b8a6' },
    { label: 'Coral', hex: '#fb923c' },
  ];

  function close() {
    dispatch('close');
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      close();
    }
  }

  async function handleSubmit() {
    const trimmed = name.trim();
    if (!trimmed) {
      errorMsg = 'Shelf name cannot be empty.';
      return;
    }
    errorMsg = '';
    isSaving = true;

    try {
      if (shelf && shelf.id) {
        await updateShelf(shelf.id, trimmed, description.trim(), color);
        shelves.update(list => list.map(s => s.id === shelf.id ? { ...s, name: trimmed, description: description.trim(), color } : s));
        dispatch('saved', { id: shelf.id, name: trimmed, description: description.trim(), color });
      } else {
        const newShelf = await createShelf(trimmed, description.trim(), color);
        if (newShelf && newShelf.id) {
          shelves.update(list => [...list, newShelf]);
          dispatch('created', newShelf);
        }
      }
      close();
    } catch (err) {
      console.error('Failed to save shelf:', err);
      errorMsg = 'Failed to save shelf. Please try again.';
    } finally {
      isSaving = false;
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div
  class="modal-backdrop"
  on:click={close}
  role="dialog"
  aria-modal="true"
  aria-labelledby="shelf-modal-title"
  tabindex="-1"
>
  <div class="modal-box" on:click|stopPropagation role="document">
    <div class="modal-header">
      <div class="header-left">
        <div class="shelf-icon-badge" style="background: {color}20; color: {color};">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
            <path d="M9 10h6"/>
          </svg>
        </div>
        <h2 id="shelf-modal-title">{shelf ? 'Edit Shelf' : 'New Shelf'}</h2>
      </div>
      <button class="close-btn" on:click={close} aria-label="Close modal">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 6 6 18"/><path d="m6 6 12 12"/>
        </svg>
      </button>
    </div>

    <form on:submit|preventDefault={handleSubmit} class="modal-body">
      {#if errorMsg}
        <div class="error-banner">{errorMsg}</div>
      {/if}

      <div class="form-group">
        <label for="shelf-name-input">Shelf Name</label>
        <input
          id="shelf-name-input"
          type="text"
          class="input-text"
          placeholder="e.g. Science Fiction, Work, Summer 2026..."
          bind:value={name}
          required
        />
      </div>

      <div class="form-group">
        <label for="shelf-desc-input">Description <span class="optional-label">(Optional)</span></label>
        <input
          id="shelf-desc-input"
          type="text"
          class="input-text"
          placeholder="What belongs on this shelf?"
          bind:value={description}
        />
      </div>

      <div class="form-group">
        <label>Shelf Color</label>
        <div class="color-palette">
          {#each PRESET_COLORS as c}
            <button
              type="button"
              class="color-dot"
              class:selected={color === c.hex}
              style="background-color: {c.hex};"
              title={c.label}
              on:click={() => color = c.hex}
            >
              {#if color === c.hex}
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
              {/if}
            </button>
          {/each}
        </div>
      </div>

      <!-- Live Preview -->
      <div class="preview-wrap">
        <span class="preview-label">Preview:</span>
        <div class="shelf-pill-preview" style="border-left-color: {color};">
          <span class="shelf-dot" style="background-color: {color};"></span>
          <span class="shelf-pill-name">{name.trim() || 'Untitled Shelf'}</span>
        </div>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn btn-secondary" on:click={close} disabled={isSaving}>
          Cancel
        </button>
        <button type="submit" class="btn btn-primary" disabled={isSaving || !name.trim()}>
          {isSaving ? 'Saving...' : shelf ? 'Save Changes' : 'Create Shelf'}
        </button>
      </div>
    </form>
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
    align-items: center;
    justify-content: space-between;
    padding: var(--space-md) var(--space-lg);
    border-bottom: 1px solid var(--border-subtle);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .shelf-icon-badge {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
  }

  .modal-header h2 {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--fg-primary);
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
    padding: var(--space-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  .error-banner {
    background: rgba(239, 68, 68, 0.15);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171;
    font-size: 0.8125rem;
    padding: var(--space-sm) var(--space-md);
    border-radius: var(--radius-sm);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
  }

  .form-group label {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--fg-secondary);
  }

  .optional-label {
    font-weight: 400;
    color: var(--fg-tertiary);
  }

  .input-text {
    background: var(--bg-primary);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-sm);
    padding: 8px 12px;
    font-size: 0.875rem;
    font-family: var(--font-sans);
    color: var(--fg-primary);
    transition: border-color var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out);
  }

  .input-text:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-subtle);
  }

  .color-palette {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    flex-wrap: wrap;
    padding-top: 4px;
  }

  .color-dot {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: transform var(--duration-fast) var(--ease-out),
                box-shadow var(--duration-fast) var(--ease-out);
    padding: 0;
  }

  .color-dot:hover {
    transform: scale(1.15);
  }

  .color-dot.selected {
    box-shadow: 0 0 0 2px var(--bg-card), 0 0 0 4px currentColor;
    transform: scale(1.1);
  }

  .preview-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    padding: var(--space-xs) 0;
  }

  .preview-label {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .shelf-pill-preview {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-left: 3px solid;
    border-radius: var(--radius-sm);
    padding: 4px 10px;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--fg-primary);
  }

  .shelf-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }

  .shelf-pill-name {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-sm);
    margin-top: var(--space-sm);
  }
</style>
