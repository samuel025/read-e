<script>
  import {
    highlights, currentBook, currentBookId, currentSpineIndex,
    currentChapter, toc
  } from '../stores/app.js';
  import {
    getHighlights, deleteHighlight, updateHighlightNote,
    getChapter, saveProgress
  } from './api.js';

  let selectedColor = 'all';
  let editingNoteId = null;
  let noteDraft = '';
  let showExportModal = false;
  let copied = false;

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

    if ($currentBook?.format === 'pdf') {
      currentSpineIndex.set(h.spineIndex);
      window.dispatchEvent(new CustomEvent('pdf-jump-to-highlight', {
        detail: h
      }));
      return;
    }

    const switchedChapter = $currentSpineIndex !== h.spineIndex;
    if (switchedChapter) {
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
          text: h.text,
        }, '*');
      }
    }, switchedChapter ? 300 : 30);
  }

  async function handleDelete(e, id) {
    e.stopPropagation();
    await deleteHighlight(id);
    highlights.update(items => items.filter(item => item.id !== id));

    if ($currentBook?.format === 'pdf') {
      window.dispatchEvent(new CustomEvent('pdf-remove-highlight', {
        detail: { highlightId: id }
      }));
      return;
    }

    const iframe = document.querySelector('.chapter-frame');
    if (iframe?.contentWindow) {
      iframe.contentWindow.postMessage({
        type: 'remove-highlight-mark',
        highlightId: id,
      }, '*');
    }
  }

  function startEditNote(e, h) {
    e.stopPropagation();
    editingNoteId = h.id;
    noteDraft = h.note || '';
  }

  function cancelEditNote(e) {
    e?.stopPropagation();
    editingNoteId = null;
    noteDraft = '';
  }

  async function saveNote(e, id) {
    e?.stopPropagation();
    const cleanNote = noteDraft.trim();
    await updateHighlightNote(id, cleanNote);
    highlights.update(items =>
      items.map(item => item.id === id ? { ...item, note: cleanNote } : item)
    );
    editingNoteId = null;
    noteDraft = '';
  }

  function generateMarkdown() {
    const bookTitle = $currentBook?.title || 'Book Highlights';
    const author = $currentBook?.author || '';

    let md = `# Highlights from ${bookTitle}\n`;
    if (author) md += `*by ${author}*\n`;
    md += `\nExported on ${new Date().toLocaleDateString()}\n\n---\n\n`;

    // Group by chapter
    const byChapter = {};
    for (const h of $highlights) {
      if (!byChapter[h.spineIndex]) {
        byChapter[h.spineIndex] = [];
      }
      byChapter[h.spineIndex].push(h);
    }

    const sortedChapters = Object.keys(byChapter).map(Number).sort((a, b) => a - b);

    for (const spineIdx of sortedChapters) {
      const title = getChapterTitle(spineIdx);
      md += `### ${title}\n\n`;

      for (const h of byChapter[spineIdx]) {
        md += `> "${h.text}"\n\n`;
        if (h.note && h.note.trim()) {
          md += `> 📝 **Note:** ${h.note.trim()}\n\n`;
        }
      }
    }

    return md;
  }

  async function copyMarkdown() {
    const text = generateMarkdown();
    await navigator.clipboard.writeText(text);
    copied = true;
    setTimeout(() => {
      copied = false;
    }, 2000);
  }

  function downloadMarkdown() {
    const text = generateMarkdown();
    const blob = new Blob([text], { type: 'text/markdown;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    const safeTitle = ($currentBook?.title || 'highlights').replace(/[^a-z0-9]/gi, '_').toLowerCase();
    link.href = url;
    link.download = `${safeTitle}_highlights.md`;
    link.click();
    URL.revokeObjectURL(url);
  }
</script>

<div class="highlights-panel">
  <!-- Color filter pills & Export action -->
  <div class="filter-bar">
    <div class="filter-chips">
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

    {#if $highlights.length > 0}
      <button
        class="export-btn"
        on:click={() => showExportModal = true}
        title="Export highlights to Markdown"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="7 10 12 15 17 10"/>
          <line x1="12" y1="15" x2="12" y2="3"/>
        </svg>
        <span>Export</span>
      </button>
    {/if}
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
          <!-- svelte-ignore a11y-no-static-element-interactions -->
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
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/>
                    <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>
                  </svg>
                </button>
              </div>
            </div>

            <p class="highlight-quote">“{h.text}”</p>

            <!-- Note section -->
            {#if editingNoteId === h.id}
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <div class="note-editor" on:click|stopPropagation>
                <textarea
                  bind:value={noteDraft}
                  placeholder="Add a thought or reflection..."
                  rows="2"
                  autoFocus
                  on:keydown={(e) => {
                    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                      saveNote(e, h.id);
                    } else if (e.key === 'Escape') {
                      cancelEditNote(e);
                    }
                  }}
                ></textarea>
                <div class="editor-actions">
                  <span class="shortcut-tip">Cmd+Enter to save</span>
                  <div class="button-group">
                    <button class="btn-cancel" on:click={(e) => cancelEditNote(e)}>Cancel</button>
                    <button class="btn-save" on:click={(e) => saveNote(e, h.id)}>Save</button>
                  </div>
                </div>
              </div>
            {:else if h.note && h.note.trim()}
              <div class="note-display" on:click={(e) => startEditNote(e, h)} role="button" tabindex="0">
                <div class="note-icon">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/>
                  </svg>
                </div>
                <span class="note-text">{h.note}</span>
              </div>
            {:else}
              <div class="add-note-row">
                <button class="add-note-btn" on:click={(e) => startEditNote(e, h)}>
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <line x1="12" y1="5" x2="12" y2="19"></line>
                    <line x1="5" y1="12" x2="19" y2="12"></line>
                  </svg>
                  <span>Add note</span>
                </button>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<!-- Export Modal -->
{#if showExportModal}
  <div class="modal-backdrop" on:click={() => showExportModal = false} role="dialog" aria-modal="true">
    <div class="export-modal" on:click|stopPropagation>
      <div class="modal-header">
        <div class="modal-title-wrap">
          <h3>Export Highlights</h3>
          <span class="modal-sub">Formatted Markdown for Obsidian, Notion, or Roam</span>
        </div>
        <button class="modal-close" on:click={() => showExportModal = false} title="Close">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>

      <div class="modal-body">
        <pre class="markdown-preview">{generateMarkdown()}</pre>
      </div>

      <div class="modal-footer">
        <button class="btn-download" on:click={downloadMarkdown}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="7 10 12 15 17 10"/>
            <line x1="12" y1="15" x2="12" y2="3"/>
          </svg>
          <span>Download .md</span>
        </button>
        <button class="btn-copy" class:copied on:click={copyMarkdown}>
          {#if copied}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
            <span>Copied to Clipboard!</span>
          {:else}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect width="14" height="14" x="8" y="8" rx="2" ry="2"/>
              <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>
            </svg>
            <span>Copy Markdown</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .highlights-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .filter-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }

  .filter-chips {
    display: flex;
    gap: 4px;
    overflow-x: auto;
  }

  .filter-chips::-webkit-scrollbar {
    display: none;
  }

  .filter-chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 7px;
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

  .export-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--accent);
    background: var(--accent-subtle);
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
    flex-shrink: 0;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .export-btn:hover {
    filter: brightness(1.1);
    transform: translateY(-0.5px);
  }

  .color-dot {
    width: 6px;
    height: 6px;
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

  /* Note styling */
  .note-display {
    margin-top: 8px;
    padding: 6px 8px;
    background: var(--bg-secondary);
    border-radius: var(--radius-xs);
    border-left: 2px solid var(--accent);
    display: flex;
    align-items: flex-start;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--fg-secondary);
    line-height: 1.4;
    cursor: pointer;
    transition: background var(--duration-fast) var(--ease-out);
  }

  .note-display:hover {
    background: var(--bg-hover);
  }

  .note-icon {
    color: var(--accent);
    flex-shrink: 0;
    margin-top: 1px;
  }

  .note-text {
    word-break: break-word;
  }

  .add-note-row {
    margin-top: 6px;
  }

  .add-note-btn {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    background: transparent;
    border: none;
    color: var(--fg-tertiary);
    font-size: 0.6875rem;
    padding: 2px 4px;
    border-radius: 4px;
    cursor: pointer;
    transition: color var(--duration-fast) var(--ease-out);
  }

  .add-note-btn:hover {
    color: var(--accent);
  }

  .note-editor {
    margin-top: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .note-editor textarea {
    width: 100%;
    padding: 6px 8px;
    font-size: 0.75rem;
    font-family: inherit;
    background: var(--bg-secondary);
    color: var(--fg-primary);
    border: 1px solid var(--accent);
    border-radius: var(--radius-xs);
    resize: vertical;
    outline: none;
  }

  .editor-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .shortcut-tip {
    font-size: 0.625rem;
    color: var(--fg-tertiary);
  }

  .button-group {
    display: flex;
    gap: 4px;
  }

  .btn-cancel, .btn-save {
    padding: 3px 8px;
    font-size: 0.6875rem;
    font-weight: 500;
    border-radius: 4px;
    cursor: pointer;
    border: none;
  }

  .btn-cancel {
    background: transparent;
    color: var(--fg-secondary);
  }

  .btn-cancel:hover {
    background: var(--bg-hover);
  }

  .btn-save {
    background: var(--accent);
    color: white;
    font-weight: 600;
  }

  .btn-save:hover {
    filter: brightness(1.1);
  }

  /* Modal Styles */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
    animation: fadeIn 0.15s ease-out;
  }

  .export-modal {
    background: var(--bg-primary);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-md);
    width: 100%;
    max-width: 600px;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 16px 40px rgba(0, 0, 0, 0.28);
    overflow: hidden;
    animation: scaleIn 0.15s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .modal-title-wrap h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
    color: var(--fg-primary);
  }

  .modal-sub {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
    margin-top: 2px;
  }

  .modal-close {
    background: transparent;
    border: none;
    color: var(--fg-tertiary);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .modal-close:hover {
    color: var(--fg-primary);
    background: var(--bg-hover);
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
    background: var(--bg-secondary);
  }

  .markdown-preview {
    margin: 0;
    font-family: 'JetBrains Mono', 'Fira Code', monospace;
    font-size: 0.75rem;
    line-height: 1.5;
    color: var(--fg-secondary);
    white-space: pre-wrap;
    word-break: break-word;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 20px;
    border-top: 1px solid var(--border-subtle);
    background: var(--bg-primary);
  }

  .btn-download, .btn-copy {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 7px 14px;
    font-size: 0.8125rem;
    font-weight: 500;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .btn-download {
    background: transparent;
    border: 1px solid var(--border-subtle);
    color: var(--fg-secondary);
  }

  .btn-download:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .btn-copy {
    background: var(--accent);
    border: 1px solid var(--accent);
    color: white;
    font-weight: 600;
  }

  .btn-copy:hover {
    filter: brightness(1.1);
  }

  .btn-copy.copied {
    background: #10b981;
    border-color: #10b981;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes scaleIn {
    from { transform: scale(0.96); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }
</style>
