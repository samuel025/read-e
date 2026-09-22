<script>
  import {
    chapterPageInfo,
    currentBook,
    currentSpineIndex,
    spineCount
  } from '../stores/app.js';

  $: totalSpine = $spineCount || 1;
  $: overallPercent = Math.min(
    100,
    Math.max(0, Math.round((($currentSpineIndex + 1) / totalSpine) * 100))
  );

  $: pagesLeftText = $chapterPageInfo.pagesLeft === 0
    ? 'End of chapter'
    : `${$chapterPageInfo.pagesLeft} page${$chapterPageInfo.pagesLeft === 1 ? '' : 's'} left in chapter`;
</script>

<footer class="reader-footer" role="status" aria-label="Reading progress">
  <div class="footer-inner">
    <!-- Chapter Title (Left) -->
    <div class="footer-col footer-left" title={$chapterPageInfo.chapterTitle || 'Current chapter'}>
      <svg class="footer-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1-2.5-2.5Z"></path>
      </svg>
      <span class="footer-title">{$chapterPageInfo.chapterTitle || 'Chapter'}</span>
    </div>

    <!-- Pages Left in Chapter (Center) -->
    <div class="footer-col footer-center">
      <span class="footer-pages-left" class:end-of-chapter={$chapterPageInfo.pagesLeft === 0}>
        {pagesLeftText}
      </span>
    </div>

    <!-- Chapter & Book Progress (Right) -->
    <div class="footer-col footer-right">
      <span class="footer-page-counts">
        Page {$chapterPageInfo.currentPage} of {$chapterPageInfo.totalPages}
      </span>
      <span class="footer-bullet">•</span>
      <span class="footer-percent">{overallPercent}%</span>
    </div>
  </div>
</footer>

<style>
  .reader-footer {
    width: 100%;
    height: 32px;
    background: var(--bg-secondary);
    border-top: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    user-select: none;
    z-index: 10;
    transition: opacity var(--duration-normal), background var(--duration-normal);
  }

  .footer-inner {
    width: 100%;
    max-width: 1100px;
    padding: 0 var(--space-md);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    font-size: 0.72rem;
    color: var(--fg-tertiary);
    letter-spacing: 0.01em;
  }

  .footer-col {
    display: flex;
    align-items: center;
    gap: 6px;
    overflow: hidden;
  }

  .footer-left {
    flex: 1;
    min-width: 0;
  }

  .footer-icon {
    flex-shrink: 0;
    opacity: 0.65;
  }

  .footer-title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 500;
  }

  .footer-center {
    flex-shrink: 0;
  }

  .footer-pages-left {
    font-weight: 600;
    color: var(--accent);
    padding: 1px 8px;
    background: rgba(var(--accent-rgb, 99, 102, 241), 0.08);
    border-radius: 10px;
    letter-spacing: 0.015em;
  }

  .footer-pages-left.end-of-chapter {
    color: var(--fg-secondary);
    background: var(--bg-hover);
  }

  .footer-right {
    flex: 1;
    justify-content: flex-end;
    font-variant-numeric: tabular-nums;
  }

  .footer-page-counts {
    font-weight: 500;
  }

  .footer-bullet {
    opacity: 0.4;
  }

  .footer-percent {
    font-weight: 600;
    color: var(--fg-secondary);
  }

  @media (max-width: 600px) {
    .footer-left {
      display: none;
    }
  }
</style>
