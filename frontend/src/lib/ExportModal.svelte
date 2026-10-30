<script>
  import { createEventDispatcher } from 'svelte';

  export let title = 'Export';
  export let subtitle = '';
  export let markdown = '';
  export let filename = 'export.md';

  const dispatch = createEventDispatcher();
  let copied = false;
  let copiedTimer = null;

  function close() {
    dispatch('close');
  }

  async function copyMarkdown() {
    await navigator.clipboard.writeText(markdown);
    copied = true;
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { copied = false; }, 2000);
  }

  function downloadMarkdown() {
    const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  }
</script>

<svelte:window on:keydown={(e) => { if (e.key === 'Escape') close(); }} />

<!-- Escape is handled by the window listener above. -->
<!-- svelte-ignore a11y-click-events-have-key-events -->
<div class="modal-backdrop" on:click|self={close} role="presentation">
  <div class="export-modal" role="dialog" aria-modal="true" aria-label={title}>
    <div class="modal-header">
      <div class="modal-title-wrap">
        <h3>{title}</h3>
        {#if subtitle}<span class="modal-sub">{subtitle}</span>{/if}
      </div>
      <button class="modal-close" on:click={close} title="Close">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="18" y1="6" x2="6" y2="18"></line>
          <line x1="6" y1="6" x2="18" y2="18"></line>
        </svg>
      </button>
    </div>

    <div class="modal-body">
      <pre class="markdown-preview">{markdown}</pre>
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

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
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
    transition-property: background-color, color, border-color, opacity, transform;
    transition-duration: var(--duration-fast);
    transition-timing-function: var(--ease-out);
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
