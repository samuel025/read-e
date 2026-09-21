<script>
  import { createEventDispatcher } from 'svelte';
  import { openBook, getTOC, getProgress, getChapter, getHighlights } from './api.js';
  import {
    currentBook, currentBookId, currentChapter,
    currentSpineIndex, spineCount, toc, view, highlights
  } from '../stores/app.js';

  export let book;

  const dispatch = createEventDispatcher();

  let hovering = false;

  async function handleOpen() {
    currentBookId.set(book.id);

    try {
      const info = await openBook(book.id);
      currentBook.set(info);

      if (info.format === 'pdf') {
        const highlightsData = await getHighlights(book.id);
        highlights.set(highlightsData || []);
        view.set('reader');
        return;
      }

      spineCount.set(info.spineCount);

      const tocData = await getTOC(book.id);
      toc.set(tocData || []);

      const highlightsData = await getHighlights(book.id);
      highlights.set(highlightsData || []);

      // Restore progress
      const progress = await getProgress(book.id);
      const startIndex = progress?.spineIndex || 0;
      currentSpineIndex.set(startIndex);

      const html = await getChapter(book.id, startIndex);
      currentChapter.set(html);

      view.set('reader');
    } catch (err) {
      console.error('Failed to open book:', err);
    }
  }

  function handleRemove(e) {
    e.stopPropagation();
    dispatch('remove', book.id);
  }

  // Generate a gradient fallback when no cover image
  function getGradient(title) {
    let hash = 0;
    for (let i = 0; i < title.length; i++) {
      hash = title.charCodeAt(i) + ((hash << 5) - hash);
    }
    const h1 = Math.abs(hash % 360);
    const h2 = (h1 + 40) % 360;
    return `linear-gradient(135deg, hsl(${h1}, 45%, 35%), hsl(${h2}, 55%, 25%))`;
  }
</script>

<button
  class="book-card"
  class:hovering
  on:click={handleOpen}
  on:mouseenter={() => (hovering = true)}
  on:mouseleave={() => (hovering = false)}
  id="book-{book.id}"
>
  <div class="cover-wrapper">
    {#if book.coverBase64}
      <img
        class="cover-img"
        src={book.coverBase64}
        alt="{book.title} cover"
        loading="lazy"
      />
    {:else}
      <div class="cover-placeholder" style="background: {getGradient(book.title)}">
        <span class="placeholder-title">{book.title.slice(0, 2).toUpperCase()}</span>
      </div>
    {/if}

    {#if book.format === 'pdf'}
      <div class="format-badge">PDF</div>
    {/if}

    {#if book.hasProgress}
      <div class="progress-badge">
        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
        </svg>
        Continue
      </div>
    {/if}

    <!-- Remove button - shows on hover -->
    <button
      class="remove-btn"
      class:visible={hovering}
      on:click={handleRemove}
      title="Remove from library"
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M18 6 6 18"/><path d="m6 6 12 12"/>
      </svg>
    </button>
  </div>

  <div class="book-meta">
    <span class="book-title">{book.title}</span>
    {#if book.author}
      <span class="book-author">{book.author}</span>
    {/if}
  </div>
</button>

<style>
  .book-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    cursor: pointer;
    background: none;
    border: none;
    text-align: left;
    padding: 0;
    font-family: var(--font-sans);
    transition: transform var(--duration-normal) var(--ease-out);
  }

  .book-card:active {
    transform: scale(0.97);
  }

  .cover-wrapper {
    position: relative;
    aspect-ratio: 2 / 3;
    border-radius: var(--radius-md);
    overflow: hidden;
    /* Layered shadow — not flat border */
    box-shadow: var(--shadow-md);
    transition: box-shadow var(--duration-normal) var(--ease-out),
                transform var(--duration-normal) var(--ease-out);
  }

  .book-card:hover .cover-wrapper {
    box-shadow: var(--shadow-xl);
    transform: translateY(-4px);
  }

  .cover-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .placeholder-title {
    font-size: 2rem;
    font-weight: 700;
    color: rgba(255, 255, 255, 0.6);
    letter-spacing: 0.05em;
  }

  .format-badge {
    position: absolute;
    top: 8px;
    left: 8px;
    background: rgba(15, 23, 42, 0.75);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    color: #f8fafc;
    font-size: 0.625rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    padding: 2px 6px;
    border-radius: 4px;
    border: 1px solid rgba(255, 255, 255, 0.15);
  }

  .progress-badge {
    position: absolute;
    bottom: 8px;
    left: 8px;
    display: flex;
    align-items: center;
    gap: 4px;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    color: #fff;
    font-size: 0.6875rem;
    font-weight: 500;
    padding: 3px 8px;
    border-radius: 100px;
  }

  .remove-btn {
    position: absolute;
    top: 6px;
    right: 6px;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    border: none;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    color: #fff;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    opacity: 0;
    transform: scale(0.9);
    transition: opacity var(--duration-normal) var(--ease-out),
                transform var(--duration-normal) var(--ease-out);
  }

  .remove-btn.visible {
    opacity: 1;
    transform: scale(1);
  }

  .remove-btn:hover {
    background: rgba(239, 68, 68, 0.8);
  }

  .remove-btn:active {
    transform: scale(0.9);
  }

  .book-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0 2px;
  }

  .book-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--fg-primary);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.3;
  }

  .book-author {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
    display: -webkit-box;
    -webkit-line-clamp: 1;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
