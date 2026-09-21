<script>
  import {
    view, currentBook, currentBookId, currentSpineIndex,
    spineCount, tocOpen, settings, settingsOpen
  } from '../stores/app.js';
  import { saveSettings } from './api.js';

  function goBack() {
    view.set('library');
    currentBook.set(null);
    currentBookId.set(null);
  }

  function toggleTOC() {
    tocOpen.update((v) => !v);
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
    <button class="btn btn-ghost btn-icon" on:click={toggleTOC} title="Toggle table of contents" id="toc-toggle-btn">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 12h18"/><path d="M3 6h18"/><path d="M3 18h18"/>
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
  </div>

  <div class="toolbar-right">
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
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    width: 120px;
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
    height: 20px;
    background: var(--border-subtle);
    margin: 0 var(--space-xs);
  }

  .theme-emoji {
    font-size: 1rem;
    line-height: 1;
  }
</style>
