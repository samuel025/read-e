<script>
  import { settings, settingsOpen } from '../stores/app.js';
  import { saveSettings } from './api.js';

  let panelEl;

  function close() {
    settingsOpen.set(false);
  }

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) close();
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') close();
  }

  async function setTheme(theme) {
    settings.update((s) => {
      const updated = { ...s, theme };
      saveSettings(updated);
      return updated;
    });
  }

  async function setFontSize(size) {
    settings.update((s) => {
      const updated = { ...s, fontSize: size };
      saveSettings(updated);
      return updated;
    });
  }

  async function setFontFamily(family) {
    settings.update((s) => {
      const updated = { ...s, fontFamily: family };
      saveSettings(updated);
      return updated;
    });
  }

  const fontOptions = [
    { label: 'Inter', value: "'Inter', system-ui, sans-serif" },
    { label: 'Merriweather', value: "'Merriweather', Georgia, serif" },
    { label: 'System', value: "system-ui, -apple-system, sans-serif" },
    { label: 'Monospace', value: "'JetBrains Mono', 'Fira Code', monospace" },
  ];
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events -->
<div class="settings-backdrop" on:click={handleBackdropClick} role="dialog" aria-label="Settings">
  <div class="settings-panel" bind:this={panelEl}>
    <div class="settings-header">
      <h2>Settings</h2>
      <button class="btn btn-ghost btn-icon" on:click={close} title="Close">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 6 6 18"/><path d="m6 6 12 12"/>
        </svg>
      </button>
    </div>

    <div class="settings-body">
      <!-- Theme -->
      <div class="settings-section">
        <label class="settings-label">Theme</label>
        <div class="theme-picker">
          <button
            class="theme-btn"
            class:active={$settings.theme === 'dark'}
            on:click={() => setTheme('dark')}
          >
            <div class="theme-preview dark-preview"></div>
            <span>Dark</span>
          </button>
          <button
            class="theme-btn"
            class:active={$settings.theme === 'light'}
            on:click={() => setTheme('light')}
          >
            <div class="theme-preview light-preview"></div>
            <span>Light</span>
          </button>
          <button
            class="theme-btn"
            class:active={$settings.theme === 'sepia'}
            on:click={() => setTheme('sepia')}
          >
            <div class="theme-preview sepia-preview"></div>
            <span>Sepia</span>
          </button>
        </div>
      </div>

      <!-- Font Size -->
      <div class="settings-section">
        <label class="settings-label">
          Font Size
          <span class="settings-value">{Math.round($settings.fontSize * 100)}%</span>
        </label>
        <input
          type="range"
          min="0.6"
          max="2.0"
          step="0.1"
          value={$settings.fontSize}
          on:input={(e) => setFontSize(parseFloat(e.target.value))}
          class="range-input"
        />
        <div class="range-labels">
          <span>Small</span>
          <span>Large</span>
        </div>
      </div>

      <!-- Font Family -->
      <div class="settings-section">
        <label class="settings-label">Font Family</label>
        <div class="font-picker">
          {#each fontOptions as font}
            <button
              class="font-btn"
              class:active={$settings.fontFamily === font.value}
              on:click={() => setFontFamily(font.value)}
              style="font-family: {font.value}"
            >
              {font.label}
            </button>
          {/each}
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  .settings-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: flex-end;
    z-index: 100;
    /* Fade-in: enter with ease-out */
    animation: fadeIn 200ms var(--ease-out);
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .settings-panel {
    width: 340px;
    height: 100%;
    background: var(--bg-secondary);
    border-left: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    /* Slide in from right: ease-out, start from scale(0.98) + translateX */
    animation: slideIn 250ms var(--ease-out);
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateX(20px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-md) var(--space-lg);
    border-bottom: 1px solid var(--border-subtle);
  }

  .settings-header h2 {
    font-size: 1rem;
    font-weight: 600;
  }

  .settings-body {
    padding: var(--space-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-xl);
    overflow-y: auto;
  }

  .settings-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }

  .settings-label {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--fg-secondary);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .settings-value {
    font-weight: 500;
    color: var(--fg-tertiary);
    font-variant-numeric: tabular-nums;
  }

  /* Theme picker */
  .theme-picker {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-sm);
  }

  .theme-btn {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: var(--space-sm);
    border: 2px solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: transparent;
    cursor: pointer;
    font-family: var(--font-sans);
    font-size: 0.75rem;
    color: var(--fg-secondary);
    transition: border-color var(--duration-normal) var(--ease-out),
                transform var(--duration-fast) var(--ease-out);
  }

  .theme-btn:active { transform: scale(0.97); }

  .theme-btn.active {
    border-color: var(--accent);
    color: var(--accent);
  }

  .theme-preview {
    width: 100%;
    height: 40px;
    border-radius: var(--radius-sm);
  }

  .dark-preview { background: linear-gradient(135deg, #0f1117, #1c2030); }
  .light-preview { background: linear-gradient(135deg, #ffffff, #f1f3f5); }
  .sepia-preview { background: linear-gradient(135deg, #f5f0e8, #e8dece); }

  /* Range input */
  .range-input {
    -webkit-appearance: none;
    width: 100%;
    height: 4px;
    border-radius: 2px;
    background: var(--bg-hover);
    outline: none;
  }

  .range-input::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--accent);
    cursor: pointer;
    border: 2px solid var(--bg-secondary);
    box-shadow: var(--shadow-sm);
  }

  .range-labels {
    display: flex;
    justify-content: space-between;
    font-size: 0.6875rem;
    color: var(--fg-tertiary);
  }

  /* Font picker */
  .font-picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
  }

  .font-btn {
    padding: 8px 12px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--fg-secondary);
    font-size: 0.875rem;
    cursor: pointer;
    text-align: left;
    transition: border-color var(--duration-normal) var(--ease-out),
                background var(--duration-normal) var(--ease-out),
                transform var(--duration-fast) var(--ease-out);
  }

  .font-btn:active { transform: scale(0.98); }

  .font-btn:hover {
    background: var(--bg-hover);
  }

  .font-btn.active {
    border-color: var(--accent);
    background: var(--accent-subtle);
    color: var(--accent);
  }
</style>
