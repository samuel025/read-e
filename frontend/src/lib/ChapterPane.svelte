<script>
  import { onMount, afterUpdate } from 'svelte';
  import {
    currentChapter, currentBookId, currentSpineIndex,
    spineCount, settings
  } from '../stores/app.js';
  import { getChapter, saveProgress, getProgress } from './api.js';

  let iframeEl;
  let loading = false;

  // Navigate to next/prev chapter
  async function navigate(delta) {
    const newIndex = $currentSpineIndex + delta;
    if (newIndex < 0 || newIndex >= $spineCount) return;

    loading = true;
    currentSpineIndex.set(newIndex);

    try {
      const html = await getChapter($currentBookId, newIndex);
      currentChapter.set(html);
      saveProgress($currentBookId, newIndex, 0);
    } finally {
      loading = false;
    }
  }

  // Build the theme CSS to inject into the chapter iframe
  $: themeCSS = buildThemeCSS($settings);

  function buildThemeCSS(s) {
    const themes = {
      dark: { bg: '#0f1117', fg: '#e0e2e8', link: '#818cf8', highlight: 'rgba(129, 140, 248, 0.28)' },
      light: { bg: '#ffffff', fg: '#1a1d26', link: '#6366f1', highlight: 'rgba(99, 102, 241, 0.25)' },
      sepia: { bg: '#f5f0e8', fg: '#3d2e1c', link: '#a0522d', highlight: 'rgba(160, 82, 45, 0.22)' },
      nord: { bg: '#2e3440', fg: '#eceff4', link: '#88c0d0', highlight: 'rgba(136, 192, 208, 0.28)' },
    };
    const t = themes[s.theme] || themes.dark;

    return `
      <style>
        * { box-sizing: border-box; }
        html, body {
          margin: 0;
          padding: 0;
          background: ${t.bg};
          color: ${t.fg};
          font-family: ${s.fontFamily || "'Inter', system-ui, sans-serif"};
          font-size: ${s.fontSize}em;
          line-height: 1.7;
          word-wrap: break-word;
          overflow-wrap: break-word;
          -webkit-font-smoothing: antialiased;
        }
        body {
          padding: 32px 48px;
          max-width: 800px;
          margin: 0 auto;
        }
        img, svg, video {
          max-width: 100%;
          height: auto;
        }
        a { color: ${t.link}; }
        h1, h2, h3, h4, h5, h6 {
          line-height: 1.3;
          margin-top: 1.5em;
          margin-bottom: 0.5em;
        }
        p { margin-bottom: 0.8em; }
        pre, code {
          font-size: 0.9em;
          background: rgba(128, 128, 128, 0.1);
          padding: 2px 4px;
          border-radius: 4px;
        }
        pre { padding: 12px; overflow-x: auto; }
        blockquote {
          border-left: 3px solid rgba(128, 128, 128, 0.3);
          margin: 1em 0;
          padding-left: 1em;
          color: inherit;
          opacity: 0.85;
        }
        table { border-collapse: collapse; width: 100%; }
        td, th { padding: 8px; border: 1px solid rgba(128, 128, 128, 0.2); }

        /* Footnotes & Endnotes Container */
        .footnotes,
        .endnotes,
        [epub\\:type~="footnotes"],
        [epub\\:type~="endnotes"],
        [role~="doc-footnotes"],
        [role~="doc-endnotes"],
        section[class*="footnote" i],
        section[class*="endnote" i],
        div[class*="footnotes" i],
        div[class*="endnotes" i],
        .reader-footnotes-container {
          margin-top: 3.5em !important;
          padding-top: 1.5em !important;
          border-top: 1px solid rgba(128, 128, 128, 0.25) !important;
        }

        /* Footnote / Endnote Text Elements */
        .footnotes p,
        .endnotes p,
        .footnotes li,
        .endnotes li,
        .footnotes div,
        .endnotes div,
        [epub\\:type~="footnote"],
        [epub\\:type~="endnote"],
        [role~="doc-footnote"],
        [role~="doc-endnote"],
        [class*="footnote-text" i],
        [class*="footnote_text" i],
        [class*="endnote-text" i],
        [class*="endnote_text" i],
        p[class*="footnote" i],
        p[class*="endnote" i],
        li[class*="footnote" i],
        li[class*="endnote" i],
        div[id*="footnote" i] p,
        div[id*="endnote" i] p,
        .reader-footnote,
        .reader-footnote p {
          font-size: 0.82em !important;
          line-height: 1.55 !important;
          opacity: 0.88 !important;
          margin-top: 0.35em !important;
          margin-bottom: 0.65em !important;
        }

        /* In-text footnote references (superscript in reading body) */
        sup,
        [class*="footnote-reference" i],
        [class*="footnote-ref" i],
        [class*="footnote_ref" i],
        [class*="noteref" i],
        [epub\\:type~="noteref"],
        [role~="doc-noteref"],
        a[href*="#footnote" i]:not(.reader-footnote a):not([class*="footnote-text" i] a),
        a[href*="#fn" i]:not(.reader-footnote a):not([class*="footnote-text" i] a),
        a[href*="#note" i]:not(.reader-footnote a):not([class*="footnote-text" i] a) {
          font-size: 0.72em !important;
          vertical-align: super !important;
          line-height: 0 !important;
          font-weight: 600 !important;
          text-decoration: none !important;
          padding: 0 0.15em;
          opacity: 0.92;
          cursor: pointer;
        }

        /* Number/anchor inside the footnote itself */
        .footnotes a,
        .endnotes a,
        .reader-footnote a,
        [class*="footnote-text" i] a,
        [class*="footnote_text" i] a {
          font-weight: 600 !important;
          text-decoration: none !important;
          margin-right: 0.35em !important;
          font-size: 0.9em !important;
          vertical-align: baseline !important;
        }

        /* Footnote target highlight pulse */
        @keyframes readerHighlight {
          0% { background-color: ${t.highlight}; border-radius: 4px; }
          100% { background-color: transparent; }
        }
        .reader-target-highlight {
          animation: readerHighlight 1.8s ease-out;
          border-radius: 4px;
        }

        /* Collapse empty blockquotes, empty paragraphs, and ghost spacers */
        blockquote:empty,
        p:empty,
        div:empty {
          display: none !important;
          margin: 0 !important;
          padding: 0 !important;
          border: none !important;
        }

        /* Table of Contents & Navigation List Spacing */
        .toc,
        .table-of-contents,
        [epub\\:type~="toc"],
        [role~="doc-toc"],
        nav[class*="toc" i],
        div[class*="toc" i],
        section[class*="toc" i],
        .reader-toc-page {
          line-height: 1.45 !important;
        }

        .toc p,
        .toc li,
        .toc div,
        [epub\\:type~="toc"] p,
        [role~="doc-toc"] p,
        [class*="toc" i] p,
        .reader-toc-item,
        .reader-toc-page p,
        .reader-toc-page div.calibre11 {
          margin-top: 0.35em !important;
          margin-bottom: 0.35em !important;
          line-height: 1.45 !important;
        }

        .toc a,
        [class*="toc" i] a,
        .reader-toc-item a,
        .reader-toc-page a {
          text-decoration: none !important;
        }
        .toc a:hover,
        [class*="toc" i] a:hover,
        .reader-toc-item a:hover,
        .reader-toc-page a:hover {
          text-decoration: underline !important;
        }

        /* Collapse artificial spacer divs from calibre / publishers */
        .reader-spacer-collapsed,
        .reader-toc-page .calibre16,
        .reader-toc-page .calibre10,
        .reader-toc-page .calibre28 {
          display: none !important;
          height: 0 !important;
          margin: 0 !important;
          padding: 0 !important;
        }

        @media (prefers-reduced-motion: reduce) {
          * { transition: none !important; animation: none !important; }
        }

        [style*="font-family"] { font-family: inherit !important; }
      </style>
    `;
  }

  $: srcdoc = buildSrcdoc($currentChapter, themeCSS);

  function buildSrcdoc(chapter, css) {
    if (!chapter) return '';

    const script = `
      <script>
        (function() {
          function setupContentEnhancements() {
            try {
              // 1. Clean up empty blockquotes (Kindle/Calibre indentation artifacts)
              var blockquotes = document.querySelectorAll('blockquote');
              for (var b = 0; b < blockquotes.length; b++) {
                var bq = blockquotes[b];
                if (!bq.textContent.trim()) {
                  bq.style.display = 'none';
                  bq.style.margin = '0';
                  bq.style.padding = '0';
                  bq.style.border = 'none';
                }
              }

              // 2. Collapse artificial spacer divs/p elements that contain only whitespace or &nbsp;
              var candidates = document.querySelectorAll('div, p');
              for (var c = 0; c < candidates.length; c++) {
                var el = candidates[c];
                var raw = el.textContent.replace(/[\\s\\u00a0]/g, '');
                if (!raw && !el.querySelector('img, svg, picture, video, hr, canvas, input')) {
                  if (el.classList.contains('calibre16') || el.classList.contains('calibre10') || el.classList.contains('calibre28') || el.children.length === 0) {
                    el.classList.add('reader-spacer-collapsed');
                    el.style.display = 'none';
                    el.style.height = '0';
                    el.style.margin = '0';
                    el.style.padding = '0';
                  }
                }
              }

              // 3. Detect Table of Contents pages and apply compact spacing
              var headings = document.querySelectorAll('h1, h2, h3, h4, h5, h6');
              var isTOC = false;
              for (var h = 0; h < headings.length; h++) {
                var hText = headings[h].textContent.trim();
                if (/^(contents|table of contents|brief contents|summary)$/i.test(hText)) {
                  isTOC = true;
                  break;
                }
              }

              var paragraphs = document.querySelectorAll('p, div.calibre11, li');
              if (!isTOC && paragraphs.length >= 6) {
                var withLinks = 0;
                for (var p = 0; p < paragraphs.length; p++) {
                  if (paragraphs[p].querySelector('a')) withLinks++;
                }
                if (withLinks / paragraphs.length > 0.65) {
                  isTOC = true;
                }
              }

              if (isTOC) {
                document.body.classList.add('reader-toc-page');
                for (var pi = 0; pi < paragraphs.length; pi++) {
                  if (paragraphs[pi].querySelector('a')) {
                    paragraphs[pi].classList.add('reader-toc-item');
                  }
                }
              }

              // 4. Footnote backlinks and reference targets
              var links = document.querySelectorAll('a[href*="#"]');
              for (var i = 0; i < links.length; i++) {
                var href = links[i].getAttribute('href') || '';
                var hashIdx = href.indexOf('#');
                if (hashIdx === -1) continue;
                var hash = href.substring(hashIdx + 1);
                if (!hash) continue;

                var lowerHref = href.toLowerCase();
                if (lowerHref.indexOf('footnote') !== -1 || lowerHref.indexOf('endnote') !== -1 || lowerHref.indexOf('fn') !== -1) {
                  var target = document.getElementById(hash) || document.querySelector('[name="' + CSS.escape(hash) + '"]');
                  if (target) {
                    var block = target.closest('p, li, .calibre1, [class*="footnote" i]') || target;
                    block.classList.add('reader-footnote');
                  }
                }
              }

              var containers = document.querySelectorAll('[class*="footnote" i], [class*="endnote" i], [id*="footnote" i], [id*="endnote" i]');
              for (var j = 0; j < containers.length; j++) {
                var tag = containers[j].tagName;
                if (tag === 'DIV' || tag === 'SECTION' || tag === 'ASIDE' || tag === 'OL' || tag === 'UL') {
                  containers[j].classList.add('reader-footnotes-container');
                }
              }

              document.addEventListener('click', function(e) {
                var a = e.target.closest('a');
                if (!a) return;
                var href = a.getAttribute('href') || '';
                var hashIdx = href.indexOf('#');
                if (hashIdx === -1) return;
                var hash = href.substring(hashIdx + 1);
                if (!hash) return;

                var target = document.getElementById(hash) || document.querySelector('[name="' + CSS.escape(hash) + '"]');
                if (target) {
                  e.preventDefault();
                  target.scrollIntoView({ behavior: 'smooth', block: 'center' });
                  target.classList.remove('reader-target-highlight');
                  void target.offsetWidth;
                  target.classList.add('reader-target-highlight');
                  setTimeout(function() {
                    target.classList.remove('reader-target-highlight');
                  }, 1800);
                }
              });
            } catch (err) {}
          }

          if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', setupContentEnhancements);
          } else {
            setupContentEnhancements();
          }
        })();
      </` + `script>
    `;

    const injection = css + script;
    if (chapter.includes('<head') || chapter.includes('<HEAD')) {
      return chapter.replace(/<head[^>]*>/i, '$&' + injection);
    }
    return `<!DOCTYPE html><html><head>${injection}</head><body>${chapter}</body></html>`;
  }

  // Keyboard navigation
  function handleKeydown(e) {
    if (e.key === 'ArrowRight' || e.key === 'PageDown') {
      e.preventDefault();
      navigate(1);
    } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
      e.preventDefault();
      navigate(-1);
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  });

  // Save scroll position periodically
  function handleIframeLoad() {
    if (!iframeEl?.contentWindow) return;

    // Restore scroll position
    getProgress($currentBookId).then((pos) => {
      if (pos?.scrollOffset && iframeEl?.contentWindow) {
        iframeEl.contentWindow.scrollTo(0, pos.scrollOffset);
      }
    });

    // Save scroll on interval
    const interval = setInterval(() => {
      if (iframeEl?.contentWindow) {
        const scrollY = iframeEl.contentWindow.scrollY || 0;
        saveProgress($currentBookId, $currentSpineIndex, scrollY);
      }
    }, 5000);

    // Cleanup on next load
    iframeEl.addEventListener('load', () => clearInterval(interval), { once: true });
  }
</script>

<div class="chapter-pane">
  {#if loading}
    <div class="chapter-loading">
      <div class="spinner"></div>
    </div>
  {/if}

  <iframe
    bind:this={iframeEl}
    class="chapter-frame"
    title="Chapter content"
    sandbox="allow-same-origin allow-scripts"
    srcdoc={srcdoc}
    on:load={handleIframeLoad}
  ></iframe>

  <!-- Chapter nav buttons -->
  <div class="chapter-nav">
    <button
      class="nav-btn prev"
      class:disabled={$currentSpineIndex <= 0}
      on:click={() => navigate(-1)}
      title="Previous chapter"
      id="prev-chapter-btn"
    >
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m15 18-6-6 6-6"/>
      </svg>
    </button>
    <button
      class="nav-btn next"
      class:disabled={$currentSpineIndex >= $spineCount - 1}
      on:click={() => navigate(1)}
      title="Next chapter"
      id="next-chapter-btn"
    >
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="m9 18 6-6-6-6"/>
      </svg>
    </button>
  </div>
</div>

<style>
  .chapter-pane {
    flex: 1;
    position: relative;
    overflow: hidden;
    display: flex;
  }

  .chapter-frame {
    width: 100%;
    height: 100%;
    border: none;
    background: var(--bg-primary);
  }

  .chapter-loading {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-primary);
    z-index: 10;
  }

  .spinner {
    width: 28px;
    height: 28px;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Floating chapter nav arrows */
  .chapter-nav {
    position: absolute;
    bottom: 24px;
    right: 24px;
    display: flex;
    gap: var(--space-xs);
    z-index: 5;
  }

  .nav-btn {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    border: none;
    background: var(--glass-bg);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    color: var(--fg-primary);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    box-shadow: var(--shadow-md);
    transition: transform var(--duration-fast) var(--ease-out),
                background var(--duration-normal) var(--ease-out),
                opacity var(--duration-normal) var(--ease-out);
  }

  .nav-btn:hover {
    background: var(--bg-elevated);
    box-shadow: var(--shadow-lg);
  }

  .nav-btn:active {
    transform: scale(0.93);
  }

  .nav-btn.disabled {
    opacity: 0.3;
    pointer-events: none;
  }
</style>
