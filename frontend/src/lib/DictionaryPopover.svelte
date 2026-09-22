<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { lookupWord } from './api.js';

  export let word = '';
  export let x = 0;
  export let y = 0;
  export let placement = 'top'; // 'top' or 'bottom'

  const dispatch = createEventDispatcher();

  let loading = true;
  let error = null;
  let entry = null;
  let popoverEl;
  let isPlayingAudio = false;
  let copied = false;
  let searchWord = '';

  let posX = x;
  let posY = y;

  $: if (word) {
    searchWord = word;
    fetchDefinition(word);
  }

  async function fetchDefinition(w) {
    if (!w || !w.trim()) return;
    loading = true;
    error = null;
    entry = null;

    try {
      const res = await lookupWord(w.trim());
      if (!res) {
        error = `No definition found for "${w}"`;
      } else {
        entry = res;
      }
    } catch (err) {
      error = err.message || `No definition found for "${w}"`;
    } finally {
      loading = false;
      adjustPosition();
    }
  }

  function handleSearchSubmit(e) {
    e.preventDefault();
    if (searchWord && searchWord.trim()) {
      fetchDefinition(searchWord.trim());
    }
  }

  function playAudio() {
    if (isPlayingAudio) return;

    // Look for phonetic audio URL
    let audioUrl = '';
    if (entry?.phonetics?.length) {
      const found = entry.phonetics.find(p => p.audio && p.audio.trim());
      if (found) audioUrl = found.audio.trim();
    }

    if (audioUrl) {
      isPlayingAudio = true;
      const audio = new Audio(audioUrl);
      audio.onended = () => { isPlayingAudio = false; };
      audio.onerror = () => {
        isPlayingAudio = false;
        speakFallback();
      };
      audio.play().catch(() => {
        isPlayingAudio = false;
        speakFallback();
      });
    } else {
      speakFallback();
    }
  }

  function speakFallback() {
    if ('speechSynthesis' in window && entry?.word) {
      isPlayingAudio = true;
      const utterance = new SpeechSynthesisUtterance(entry.word);
      utterance.rate = 0.9;
      utterance.onend = () => { isPlayingAudio = false; };
      utterance.onerror = () => { isPlayingAudio = false; };
      window.speechSynthesis.speak(utterance);
    }
  }

  function copyDefinition() {
    if (!entry) return;
    let text = `${entry.word}`;
    if (entry.phonetic) text += ` ${entry.phonetic}`;
    text += '\n\n';

    entry.meanings?.forEach((m) => {
      text += `[${m.partOfSpeech}]\n`;
      m.definitions?.forEach((d, idx) => {
        text += `${idx + 1}. ${d.definition}\n`;
        if (d.example) text += `   "${d.example}"\n`;
      });
      text += '\n';
    });

    navigator.clipboard.writeText(text.trim());
    copied = true;
    setTimeout(() => { copied = false; }, 2000);
  }

  function adjustPosition() {
    if (!popoverEl) return;
    const rect = popoverEl.getBoundingClientRect();
    const padding = 16;
    const winW = window.innerWidth;
    const winH = window.innerHeight;

    let targetX = x;
    let targetY = y;

    // Horizontally clamp inside screen
    const halfW = rect.width / 2;
    if (targetX - halfW < padding) {
      targetX = halfW + padding;
    } else if (targetX + halfW > winW - padding) {
      targetX = winW - padding - halfW;
    }

    // Vertically flip or clamp
    if (placement === 'top') {
      if (targetY - rect.height < padding) {
        // Not enough room above, flip below
        targetY = targetY + 28;
      } else {
        targetY = targetY - rect.height - 10;
      }
    } else {
      if (targetY + rect.height > winH - padding) {
        // Not enough room below, flip above
        targetY = targetY - rect.height - 10;
      } else {
        targetY = targetY + 12;
      }
    }

    posX = Math.round(targetX);
    posY = Math.round(Math.max(padding, Math.min(targetY, winH - rect.height - padding)));
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      dispatch('close');
    }
  }

  function handleClickOutside(e) {
    if (popoverEl && !popoverEl.contains(e.target)) {
      dispatch('close');
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
    // Timeout so current click doesn't close immediately
    const timer = setTimeout(() => {
      window.addEventListener('mousedown', handleClickOutside);
    }, 80);
    adjustPosition();

    return () => {
      clearTimeout(timer);
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('mousedown', handleClickOutside);
    };
  });
</script>

<div
  class="dict-popover"
  bind:this={popoverEl}
  style="top: {posY}px; left: {posX}px;"
  role="dialog"
  aria-label="Dictionary definition"
  tabindex="-1"
  on:mousedown|stopPropagation
>
  <!-- Header -->
  <div class="dict-header">
    <div class="dict-title-row">
      <div class="dict-word-wrap">
        <h3 class="dict-word">{entry?.word || word}</h3>
        {#if entry?.phonetic}
          <span class="dict-phonetic">{entry.phonetic}</span>
        {/if}
      </div>

      <div class="dict-actions">
        {#if entry}
          <button
            class="dict-icon-btn"
            class:speaking={isPlayingAudio}
            on:click={playAudio}
            title="Pronounce word"
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
              <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path>
            </svg>
          </button>
        {/if}

        <button
          class="dict-close-btn"
          on:click={() => dispatch('close')}
          title="Close dictionary"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>
    </div>

    <!-- Quick search within popover -->
    <form class="dict-search-form" on:submit={handleSearchSubmit}>
      <input
        type="text"
        class="dict-search-input"
        placeholder="Search another word..."
        bind:value={searchWord}
      />
      {#if searchWord && searchWord.trim() !== (entry?.word || word)}
        <button type="submit" class="dict-search-btn">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
        </button>
      {/if}
    </form>
  </div>

  <!-- Content -->
  <div class="dict-body">
    {#if loading}
      <div class="dict-state">
        <div class="dict-spinner"></div>
        <span>Looking up definition...</span>
      </div>
    {:else if error}
      <div class="dict-state dict-error">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
        <p class="dict-err-title">{error}</p>
        <p class="dict-err-hint">Check your spelling or make sure you are connected to the internet.</p>
      </div>
    {:else if entry && entry.meanings && entry.meanings.length > 0}
      <div class="dict-meanings">
        {#each entry.meanings as meaning}
          <div class="dict-meaning-group">
            <div class="dict-pos-badge">{meaning.partOfSpeech}</div>

            <ol class="dict-def-list">
              {#each meaning.definitions as def, i}
                <li class="dict-def-item">
                  <span class="dict-def-text">{def.definition}</span>
                  {#if def.example}
                    <div class="dict-example">"{def.example}"</div>
                  {/if}
                  {#if def.synonyms && def.synonyms.length > 0}
                    <div class="dict-synonyms">
                      <span class="dict-syn-label">Synonyms:</span>
                      {#each def.synonyms.slice(0, 5) as syn}
                        <button
                          type="button"
                          class="dict-syn-tag"
                          on:click={() => fetchDefinition(syn)}
                        >
                          {syn}
                        </button>
                      {/each}
                    </div>
                  {/if}
                </li>
              {/each}
            </ol>
          </div>
        {/each}
      </div>
    {:else}
      <div class="dict-state">
        <span>No definitions available.</span>
      </div>
    {/if}
  </div>

  <!-- Footer -->
  {#if entry && !loading}
    <div class="dict-footer">
      <div class="dict-footer-status">
        {#if entry.cached}
          <span class="dict-cache-pill" title="Saved locally in SQLite">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M20 6L9 17l-5-5"></path>
            </svg>
            Offline cached
          </span>
        {/if}
      </div>

      <button
        class="dict-copy-btn"
        class:copied
        on:click={copyDefinition}
        title="Copy definition to clipboard"
      >
        {#if copied}
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M20 6L9 17l-5-5"></path>
          </svg>
          Copied!
        {:else}
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
          </svg>
          Copy
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .dict-popover {
    position: fixed;
    transform: translateX(-50%);
    width: 360px;
    max-width: calc(100vw - 32px);
    max-height: 440px;
    display: flex;
    flex-direction: column;
    background: var(--bg-surface, #1e1e24);
    color: var(--text-main, #e4e4e7);
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.12));
    border-radius: 14px;
    box-shadow: 0 18px 48px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.06);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    z-index: 10000;
    overflow: hidden;
    animation: dictPop 0.18s cubic-bezier(0.16, 1, 0.3, 1) forwards;
    font-family: inherit;
  }

  @keyframes dictPop {
    from {
      opacity: 0;
      transform: translateX(-50%) scale(0.95);
    }
    to {
      opacity: 1;
      transform: translateX(-50%) scale(1);
    }
  }

  .dict-header {
    padding: 14px 16px 10px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));
    background: var(--bg-surface-elevated, rgba(255, 255, 255, 0.02));
  }

  .dict-title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 8px;
  }

  .dict-word-wrap {
    display: flex;
    align-items: baseline;
    gap: 8px;
    flex-wrap: wrap;
    min-width: 0;
  }

  .dict-word {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text-main, #ffffff);
    letter-spacing: -0.01em;
    text-transform: capitalize;
  }

  .dict-phonetic {
    font-size: 0.85rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    color: var(--accent, #6366f1);
    background: rgba(99, 102, 241, 0.1);
    padding: 2px 6px;
    border-radius: 6px;
  }

  .dict-actions {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .dict-icon-btn, .dict-close-btn {
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text-muted, #a1a1aa);
    padding: 6px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s ease;
  }

  .dict-icon-btn:hover, .dict-close-btn:hover {
    color: var(--text-main, #ffffff);
    background: var(--bg-hover, rgba(255, 255, 255, 0.08));
  }

  .dict-icon-btn.speaking {
    color: var(--accent, #6366f1);
    animation: pulseSpeaker 0.6s infinite alternate;
  }

  @keyframes pulseSpeaker {
    from { transform: scale(1); }
    to { transform: scale(1.15); }
  }

  .dict-search-form {
    position: relative;
    display: flex;
    align-items: center;
  }

  .dict-search-input {
    width: 100%;
    padding: 5px 28px 5px 10px;
    font-size: 0.82rem;
    background: var(--bg-input, rgba(0, 0, 0, 0.2));
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));
    border-radius: 8px;
    color: var(--text-main, #ffffff);
    outline: none;
    transition: border-color 0.15s ease;
  }

  .dict-search-input:focus {
    border-color: var(--accent, #6366f1);
  }

  .dict-search-btn {
    position: absolute;
    right: 4px;
    background: transparent;
    border: none;
    color: var(--text-muted, #a1a1aa);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
  }

  .dict-search-btn:hover {
    color: var(--text-main, #ffffff);
  }

  .dict-body {
    padding: 14px 16px;
    overflow-y: auto;
    max-height: 280px;
    scrollbar-width: thin;
  }

  .dict-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 24px 12px;
    color: var(--text-muted, #a1a1aa);
    text-align: center;
    gap: 8px;
    font-size: 0.88rem;
  }

  .dict-spinner {
    width: 20px;
    height: 20px;
    border: 2px solid rgba(255, 255, 255, 0.15);
    border-top-color: var(--accent, #6366f1);
    border-radius: 50%;
    animation: dictSpin 0.7s linear infinite;
  }

  @keyframes dictSpin {
    to { transform: rotate(360deg); }
  }

  .dict-error {
    color: #ef4444;
  }

  .dict-err-title {
    margin: 4px 0 0;
    font-weight: 600;
    color: var(--text-main, #ffffff);
  }

  .dict-err-hint {
    margin: 0;
    font-size: 0.78rem;
    color: var(--text-muted, #a1a1aa);
  }

  .dict-meanings {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .dict-meaning-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .dict-pos-badge {
    align-self: flex-start;
    font-size: 0.72rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--accent, #6366f1);
    background: rgba(99, 102, 241, 0.12);
    border: 1px solid rgba(99, 102, 241, 0.2);
    padding: 2px 8px;
    border-radius: 6px;
  }

  .dict-def-list {
    margin: 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .dict-def-item {
    font-size: 0.88rem;
    line-height: 1.45;
    color: var(--text-main, #e4e4e7);
  }

  .dict-def-text {
    font-weight: 500;
  }

  .dict-example {
    margin-top: 3px;
    font-size: 0.82rem;
    color: var(--text-muted, #a1a1aa);
    font-style: italic;
  }

  .dict-synonyms {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: 4px;
    font-size: 0.75rem;
  }

  .dict-syn-label {
    color: var(--text-muted, #a1a1aa);
  }

  .dict-syn-tag {
    background: var(--bg-hover, rgba(255, 255, 255, 0.08));
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.06));
    color: var(--text-main, #e4e4e7);
    padding: 1px 6px;
    border-radius: 4px;
    font-size: 0.75rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .dict-syn-tag:hover {
    background: var(--accent, #6366f1);
    color: #ffffff;
  }

  .dict-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 14px;
    background: var(--bg-surface-elevated, rgba(255, 255, 255, 0.02));
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.08));
    font-size: 0.76rem;
  }

  .dict-cache-pill {
    display: flex;
    align-items: center;
    gap: 4px;
    color: #10b981;
    font-size: 0.72rem;
  }

  .dict-copy-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    background: transparent;
    border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    color: var(--text-muted, #a1a1aa);
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 0.76rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .dict-copy-btn:hover {
    background: var(--bg-hover, rgba(255, 255, 255, 0.08));
    color: var(--text-main, #ffffff);
  }

  .dict-copy-btn.copied {
    color: #10b981;
    border-color: rgba(16, 185, 129, 0.3);
  }
</style>
