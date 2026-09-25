<script>
  import { onMount, onDestroy } from 'svelte';
  import {
    currentChapter, currentBookId, currentSpineIndex,
    spineCount, settings, highlights, bookmarks, searchOpen,
    toc, chapterPageInfo
  } from '../stores/app.js';
  import {
    getChapter, saveProgress, getProgress,
    addHighlight, deleteHighlight, updateHighlightNote,
    saveBookmark, deleteBookmark, getHighlights
  } from './api.js';
  import DictionaryPopover from './DictionaryPopover.svelte';

  let iframeEl;
  let loading = false;
  let activeDictionary = null;

  async function navigate(delta) {
    const newIndex = $currentSpineIndex + delta;
    if (newIndex < 0 || ($spineCount > 0 && newIndex >= $spineCount)) return;

    loading = true;
    try {
      const html = await getChapter($currentBookId, newIndex);
      currentSpineIndex.set(newIndex);
      currentChapter.set(html);
      saveProgress($currentBookId, newIndex, 0);
    } catch (err) {
      console.error('Failed to navigate chapter:', err);
    } finally {
      loading = false;
    }
  }

  async function toggleCurrentBookmark() {
    const bookId = $currentBookId;
    const spineIdx = $currentSpineIndex;
    if (!bookId) return;

    const currentBm = $bookmarks.find(b => b.spineIndex === spineIdx);
    if (currentBm) {
      await deleteBookmark(currentBm.id);
      bookmarks.update(items => items.filter(b => b.id !== currentBm.id));
    } else {
      const scrollY = iframeEl?.contentWindow?.scrollY || 0;
      const newBm = {
        id: crypto.randomUUID ? crypto.randomUUID() : 'bm_' + Date.now(),
        bookId: bookId,
        spineIndex: spineIdx,
        title: `Chapter ${spineIdx + 1}`,
        scrollOffset: scrollY,
        createdAt: new Date().toISOString()
      };
      await saveBookmark(newBm);
      bookmarks.update(items => [newBm, ...items]);
    }
  }

  // Load highlights for book if not already loaded
  $: if ($currentBookId) {
    getHighlights($currentBookId).then((items) => {
      highlights.set(items || []);
    });
  }

  let navigationGeneration = 0;
  let handledNavigationGeneration = -1;

  $: themeCSS = buildThemeCSS($settings);

  // NOTE: srcdoc does NOT depend on $highlights so highlighting does not reload the iframe!
  $: srcdoc = buildSrcdoc($currentChapter, themeCSS, $currentBookId, $currentSpineIndex, navigationGeneration);

  // Synchronize highlights to iframe whenever $highlights changes
  $: if (iframeEl?.contentWindow && $currentBookId) {
    const chHighlights = $highlights.filter(h => h.bookId === $currentBookId && h.spineIndex === $currentSpineIndex);
    iframeEl.contentWindow.postMessage({
      type: 'sync-highlights',
      highlights: chHighlights
    }, '*');
  }

  function buildThemeCSS(s) {
    const themes = {
      dark: { bg: '#0f1117', fg: '#e0e2e8', link: '#818cf8', highlight: 'rgba(129, 140, 248, 0.28)' },
      light: { bg: '#ffffff', fg: '#1a1d26', link: '#6366f1', highlight: 'rgba(99, 102, 241, 0.25)' },
      sepia: { bg: '#f5f0e8', fg: '#3d2e1c', link: '#a0522d', highlight: 'rgba(160, 82, 45, 0.22)' },
      nord: { bg: '#2e3440', fg: '#eceff4', link: '#88c0d0', highlight: 'rgba(136, 192, 208, 0.28)' },
    };
    const t = themes[s.theme] || themes.dark;
    const isDark = s.theme === 'dark' || s.theme === 'nord';
    const maxWidth = s.maxWidth || 780;
    const lineHeight = s.lineHeight || 1.7;
    const textAlign = s.textAlign || 'left';

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
          line-height: ${lineHeight};
          text-align: ${textAlign};
          ${textAlign === 'justify' ? 'hyphens: auto; -webkit-hyphens: auto; text-justify: inter-word;' : ''}
          word-wrap: break-word;
          overflow-wrap: break-word;
          -webkit-font-smoothing: antialiased;
        }
        body {
          padding: 32px 48px;
          max-width: ${maxWidth}px;
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
          margin-top: 0.4em !important;
          margin-bottom: 0.4em !important;
        }

        /* Footnote In-Text Reference Super / Link */
        a[epub\\:type~="noteref"],
        a[role~="doc-noteref"],
        a[class*="noteref" i],
        a[class*="footnote" i],
        a[class*="endnote" i],
        sup a,
        a sup,
        .reader-footnote-ref {
          font-size: 0.75em !important;
          line-height: 1 !important;
          vertical-align: super !important;
          text-decoration: none !important;
          padding: 1px 4px !important;
          margin: 0 1px !important;
          border-radius: 3px !important;
          color: ${t.link} !important;
          font-weight: 600 !important;
          transition: background-color 0.15s ease, color 0.15s ease;
        }

        a[epub\\:type~="noteref"]:hover,
        a[role~="doc-noteref"]:hover,
        a[class*="noteref" i]:hover,
        sup a:hover,
        a sup:hover,
        .reader-footnote-ref:hover {
          background-color: rgba(129, 140, 248, 0.18) !important;
        }

        /* Animated Target Focus Highlight */
        .reader-target-highlight {
          animation: readerTargetPulse 1.8s cubic-bezier(0.2, 0.8, 0.2, 1) forwards !important;
          border-radius: 4px !important;
        }

        @keyframes readerTargetPulse {
          0% {
            background-color: rgba(129, 140, 248, 0.45);
            outline: 2px solid rgba(129, 140, 248, 0.6);
          }
          70% {
            background-color: rgba(129, 140, 248, 0.2);
            outline: 2px solid rgba(129, 140, 248, 0.25);
          }
          100% {
            background-color: transparent;
            outline: 2px solid transparent;
          }
        }

        /* Table of contents page styling */
        .reader-toc-page p,
        .reader-toc-page div.calibre11,
        .reader-toc-page li,
        .reader-toc-item {
          margin-top: 0.35em !important;
          margin-bottom: 0.35em !important;
          line-height: 1.45 !important;
        }
        .reader-toc-page a,
        .reader-toc-item a {
          text-decoration: none !important;
        }
        .reader-toc-page a:hover,
        .reader-toc-item a:hover {
          text-decoration: underline !important;
        }

        /* Collapse artificial spacer divs */
        .reader-spacer-collapsed,
        .reader-toc-page .calibre16,
        .reader-toc-page .calibre10,
        .reader-toc-page .calibre28 {
          display: none !important;
          height: 0 !important;
          margin: 0 !important;
          padding: 0 !important;
        }

        /* ==================== MULTI-COLOR HIGHLIGHTING ==================== */
        mark.reader-highlight {
          border-radius: 3px;
          padding: 1px 2px;
          cursor: pointer;
          transition: filter 0.15s ease, box-shadow 0.15s ease;
          display: inline;
        }
        mark.reader-highlight:hover {
          filter: brightness(0.92);
        }

        mark.reader-highlight-yellow {
          background-color: ${isDark ? 'rgba(234, 179, 8, 0.45)' : 'rgba(254, 240, 138, 0.78)'};
          color: inherit;
        }
        mark.reader-highlight-green {
          background-color: ${isDark ? 'rgba(34, 197, 94, 0.42)' : 'rgba(187, 247, 208, 0.78)'};
          color: inherit;
        }
        mark.reader-highlight-blue {
          background-color: ${isDark ? 'rgba(59, 130, 246, 0.42)' : 'rgba(191, 219, 254, 0.78)'};
          color: inherit;
        }
        mark.reader-highlight-purple {
          background-color: ${isDark ? 'rgba(168, 85, 247, 0.42)' : 'rgba(233, 213, 255, 0.78)'};
          color: inherit;
        }
        mark.reader-highlight-pink {
          background-color: ${isDark ? 'rgba(244, 63, 94, 0.42)' : 'rgba(254, 205, 211, 0.78)'};
          color: inherit;
        }

        /* Floating Selection Menu */
        .reader-selection-menu {
          position: absolute;
          z-index: 10000;
          display: flex;
          align-items: center;
          gap: 7px;
          padding: 6px 10px;
          background: ${isDark ? '#1f2430' : '#ffffff'};
          border: 1px solid ${isDark ? 'rgba(255,255,255,0.14)' : 'rgba(0,0,0,0.12)'};
          border-radius: 28px;
          box-shadow: 0 10px 28px rgba(0, 0, 0, 0.28), 0 2px 8px rgba(0, 0, 0, 0.08);
          user-select: none;
          pointer-events: auto;
          animation: menuPopIn 0.16s cubic-bezier(0.16, 1, 0.3, 1);
        }

        @keyframes menuPopIn {
          0% { transform: scale(0.85) translateY(4px); opacity: 0; }
          100% { transform: scale(1) translateY(0); opacity: 1; }
        }

        .reader-color-btn {
          width: 22px;
          height: 22px;
          border-radius: 50%;
          border: 2px solid transparent;
          cursor: pointer;
          padding: 0;
          transition: transform 0.12s ease, border-color 0.12s ease;
          display: flex;
          align-items: center;
          justify-content: center;
        }
        .reader-color-btn:hover {
          transform: scale(1.25);
          border-color: ${isDark ? '#ffffff' : '#111827'};
        }

        .reader-menu-divider {
          width: 1px;
          height: 16px;
          background: ${isDark ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.12)'};
          margin: 0 2px;
        }

        .reader-icon-btn {
          background: transparent;
          border: none;
          color: ${isDark ? '#cbd5e1' : '#475569'};
          cursor: pointer;
          padding: 3px;
          border-radius: 4px;
          display: flex;
          align-items: center;
          justify-content: center;
          transition: transform 0.12s ease, color 0.12s ease;
        }
        .reader-icon-btn:hover {
          transform: scale(1.15);
          color: ${t.link};
        }

        .reader-delete-btn {
          background: transparent;
          border: none;
          color: #ef4444;
          cursor: pointer;
          padding: 2px;
          border-radius: 4px;
          display: flex;
          align-items: center;
          justify-content: center;
          transition: transform 0.12s ease, opacity 0.12s ease;
          opacity: 0.85;
        }
        .reader-delete-btn:hover {
          transform: scale(1.15);
          opacity: 1;
        }

        /* Floating Note Popover editor */
        .reader-inline-note-box {
          position: absolute;
          z-index: 10001;
          width: 260px;
          padding: 10px;
          background: ${isDark ? '#1a1e28' : '#ffffff'};
          border: 1px solid ${isDark ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.12)'};
          border-radius: 10px;
          box-shadow: 0 12px 28px rgba(0, 0, 0, 0.28);
          display: flex;
          flex-direction: column;
          gap: 6px;
          animation: menuPopIn 0.15s ease-out;
        }
        .reader-inline-note-box textarea {
          width: 100%;
          border: 1px solid ${isDark ? 'rgba(255,255,255,0.1)' : '#cbd5e1'};
          background: ${isDark ? '#0f1117' : '#f8fafc'};
          color: ${t.fg};
          font-family: inherit;
          font-size: 0.75rem;
          padding: 6px;
          border-radius: 6px;
          resize: none;
          outline: none;
        }
        .reader-inline-note-box .note-btn-row {
          display: flex;
          justify-content: flex-end;
          gap: 4px;
        }
        .reader-inline-note-box button {
          font-size: 0.6875rem;
          padding: 3px 8px;
          border-radius: 4px;
          border: none;
          cursor: pointer;
        }
        .reader-inline-note-box .btn-save-note {
          background: ${t.link};
          color: white;
          font-weight: 600;
        }

        /* Footnote Floating Popover Card */
        .reader-footnote-popover {
          position: absolute;
          z-index: 10002;
          max-width: 400px;
          min-width: 250px;
          background: ${isDark ? '#1a1e28' : '#ffffff'};
          color: ${t.fg};
          border: 1px solid ${isDark ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.14)'};
          border-radius: 12px;
          box-shadow: 0 16px 36px rgba(0, 0, 0, 0.35), 0 2px 8px rgba(0, 0, 0, 0.1);
          padding: 12px 14px;
          font-size: 0.8125rem;
          line-height: 1.5;
          animation: popoverFadeIn 0.18s cubic-bezier(0.16, 1, 0.3, 1);
        }
        @keyframes popoverFadeIn {
          from { opacity: 0; transform: scale(0.95) translateY(4px); }
          to { opacity: 1; transform: scale(1) translateY(0); }
        }
        .reader-footnote-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          margin-bottom: 6px;
          padding-bottom: 4px;
          border-bottom: 1px solid ${isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.08)'};
        }
        .reader-footnote-title {
          font-size: 0.6875rem;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.04em;
          color: ${t.link};
          display: flex;
          align-items: center;
          gap: 4px;
        }
        .reader-footnote-close {
          background: transparent;
          border: none;
          color: ${isDark ? 'rgba(255,255,255,0.5)' : 'rgba(0,0,0,0.4)'};
          cursor: pointer;
          padding: 2px 5px;
          border-radius: 4px;
          font-size: 0.75rem;
          line-height: 1;
        }
        .reader-footnote-close:hover {
          background: ${isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.06)'};
          color: ${t.fg};
        }
        .reader-footnote-body {
          max-height: 220px;
          overflow-y: auto;
          font-size: 0.8125rem;
          opacity: 0.95;
        }
        .reader-footnote-footer {
          margin-top: 8px;
          padding-top: 6px;
          border-top: 1px solid ${isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'};
          text-align: right;
        }
        .reader-footnote-jump {
          font-size: 0.6875rem;
          color: ${t.link};
          text-decoration: none;
          cursor: pointer;
        }
        .reader-footnote-jump:hover {
          text-decoration: underline;
        }

        /* Search Match Pulse */
        .reader-search-match {
          background-color: rgba(245, 158, 11, 0.45);
          box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.25);
          border-radius: 3px;
          animation: searchMatchPulse 2.5s ease-out forwards;
        }
        @keyframes searchMatchPulse {
          0% { transform: scale(1.05); background-color: rgba(245, 158, 11, 0.8); }
          40% { transform: scale(1.02); background-color: rgba(245, 158, 11, 0.5); }
          100% { transform: scale(1); background-color: rgba(245, 158, 11, 0.25); }
        }

        @media (prefers-reduced-motion: reduce) {
          * { transition: none !important; animation: none !important; }
        }

        [style*="font-family"] { font-family: inherit !important; }
      </style>
    `;
  }

  function buildSrcdoc(chapter, css, bookId, spineIndex, navGen) {
    if (!chapter) return '';

    const script = `
      <script>
        (function() {
          var bookId = ${JSON.stringify(bookId || '')};
          var spineIndex = ${spineIndex || 0};
          var navGen = ${navGen || 0};
          var existingHighlights = [];

          var activeMenu = null;
          var activeFootnotePopover = null;
          var activeNoteBox = null;

          function removeMenu() {
            if (activeMenu && activeMenu.parentNode) {
              activeMenu.parentNode.removeChild(activeMenu);
            }
            activeMenu = null;
          }

          function removeFootnotePopover() {
            if (activeFootnotePopover && activeFootnotePopover.parentNode) {
              activeFootnotePopover.parentNode.removeChild(activeFootnotePopover);
            }
            activeFootnotePopover = null;
          }

          function removeNoteBox() {
            if (activeNoteBox && activeNoteBox.parentNode) {
              activeNoteBox.parentNode.removeChild(activeNoteBox);
            }
            activeNoteBox = null;
          }

          function isFootnoteLink(a, href) {
            var epubType = (a.getAttribute('epub:type') || '').toLowerCase();
            var role = (a.getAttribute('role') || '').toLowerCase();
            var rel = (a.getAttribute('rel') || '').toLowerCase();
            var cls = (a.className || '').toLowerCase();
            var text = a.textContent.trim();
            var lowerHref = (href || '').toLowerCase();

            if (epubType === 'noteref' || role === 'doc-noteref' || rel.indexOf('footnote') !== -1) return true;
            if (lowerHref.indexOf('footnote') !== -1 || lowerHref.indexOf('endnote') !== -1 || lowerHref.indexOf('fn') !== -1) return true;
            if (cls.indexOf('footnote') !== -1 || cls.indexOf('noteref') !== -1) return true;
            if (a.closest('sup') || a.querySelector('sup')) return true;
            if (/^\\[?\\d+\\]?$/.test(text)) return true;
            return false;
          }

          function showFootnotePopover(anchor, targetEl) {
            removeFootnotePopover();
            removeMenu();
            removeNoteBox();

            var clone = targetEl.cloneNode(true);
            var backlinks = clone.querySelectorAll('a[href*="#"], .calibre, .calibre1');
            for (var b = 0; b < backlinks.length; b++) {
              var bt = backlinks[b].textContent.trim();
              if (bt.match(/[↩^↑]/) || backlinks[b].getAttribute('role') === 'doc-backlink') {
                backlinks[b].remove();
              }
            }
            var noteHtml = clone.innerHTML.trim();

            var popover = document.createElement('div');
            popover.className = 'reader-footnote-popover';
            popover.innerHTML = [
              '<div class="reader-footnote-header">',
              '  <span class="reader-footnote-title">',
              '    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>',
              '    Footnote',
              '  </span>',
              '  <button class="reader-footnote-close" title="Close">✕</button>',
              '</div>',
              '<div class="reader-footnote-body">' + noteHtml + '</div>',
              '<div class="reader-footnote-footer">',
              '  <a href="#" class="reader-footnote-jump">Jump to endnote ↓</a>',
              '</div>'
            ].join('');

            popover.querySelector('.reader-footnote-close').addEventListener('click', function(e) {
              e.stopPropagation();
              removeFootnotePopover();
            });

            popover.querySelector('.reader-footnote-jump').addEventListener('click', function(e) {
              e.preventDefault();
              e.stopPropagation();
              removeFootnotePopover();
              targetEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
              targetEl.classList.remove('reader-target-highlight');
              void targetEl.offsetWidth;
              targetEl.classList.add('reader-target-highlight');
              setTimeout(function() {
                targetEl.classList.remove('reader-target-highlight');
              }, 1800);
            });

            document.body.appendChild(popover);
            activeFootnotePopover = popover;

            var rect = anchor.getBoundingClientRect();
            var popWidth = Math.min(380, document.body.clientWidth - 24);
            var left = rect.left + window.scrollX + (rect.width / 2) - (popWidth / 2);
            left = Math.max(12, Math.min(left, document.body.clientWidth - popWidth - 12));

            var top = rect.top + window.scrollY - popover.offsetHeight - 8;
            if (top < window.scrollY + 10) {
              top = rect.bottom + window.scrollY + 8;
            }

            popover.style.width = popWidth + 'px';
            popover.style.top = top + 'px';
            popover.style.left = left + 'px';
          }

          function getAllTextNodes(root) {
            var nodes = [];
            var walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
              acceptNode: function(node) {
                if (!node.nodeValue || !node.nodeValue.length) return NodeFilter.FILTER_REJECT;
                var p = node.parentElement;
                if (!p) return NodeFilter.FILTER_REJECT;
                var tag = p.tagName;
                if (tag === 'SCRIPT' || tag === 'STYLE' || tag === 'NOSCRIPT') return NodeFilter.FILTER_REJECT;
                if (p.closest('.reader-selection-menu') || p.closest('.reader-footnote-popover') || p.closest('.reader-inline-note-box')) return NodeFilter.FILTER_REJECT;
                return NodeFilter.FILTER_ACCEPT;
              }
            });
            var n;
            while (n = walker.nextNode()) nodes.push(n);
            return nodes;
          }

          function wrapTextNodeRange(node, start, end, id, color) {
            try {
              var range = document.createRange();
              range.setStart(node, start);
              range.setEnd(node, end);
              var mark = document.createElement('mark');
              mark.className = 'reader-highlight reader-highlight-' + color;
              mark.setAttribute('data-highlight-id', id);
              mark.setAttribute('data-color', color);
              var frag = range.extractContents();
              mark.appendChild(frag);
              range.insertNode(mark);
            } catch (e) {
              console.warn('wrapTextNodeRange error:', e);
            }
          }

          function wrapAcrossNodes(nodeMap, startIdx, endIdx, id, color) {
            if (startIdx >= endIdx || startIdx >= nodeMap.length) return;
            endIdx = Math.min(endIdx, nodeMap.length);

            var nodeSpans = [];
            var currentNode = null;
            var startOffset = 0;
            var endOffset = 0;

            for (var i = startIdx; i < endIdx; i++) {
              var item = nodeMap[i];
              if (item.node !== currentNode) {
                if (currentNode) {
                  nodeSpans.push({ node: currentNode, start: startOffset, end: endOffset });
                }
                currentNode = item.node;
                startOffset = item.offset;
                endOffset = item.offset + 1;
              } else {
                endOffset = item.offset + 1;
              }
            }
            if (currentNode) {
              nodeSpans.push({ node: currentNode, start: startOffset, end: endOffset });
            }

            for (var s = nodeSpans.length - 1; s >= 0; s--) {
              var span = nodeSpans[s];
              wrapTextNodeRange(span.node, span.start, span.end, id, color);
            }
          }

          function highlightTextInDocument(h) {
            var targetText = (h.text || '').trim();
            if (!targetText) return;

            var textNodes = getAllTextNodes(document.body);
            if (!textNodes.length) return;

            // Fast path: exact match in single text node
            for (var i = 0; i < textNodes.length; i++) {
              var node = textNodes[i];
              if (node.parentElement && node.parentElement.classList.contains('reader-highlight')) continue;
              var idx = node.nodeValue.indexOf(targetText);
              if (idx !== -1) {
                wrapTextNodeRange(node, idx, idx + targetText.length, h.id, h.color || 'yellow');
                return;
              }
            }

            // Multi-node path: continuous character mapping with whitespace normalization
            var fullText = '';
            var nodeMap = [];
            for (var n = 0; n < textNodes.length; n++) {
              var tNode = textNodes[n];
              if (tNode.parentElement && tNode.parentElement.classList.contains('reader-highlight')) continue;
              var val = tNode.nodeValue;
              for (var o = 0; o < val.length; o++) {
                nodeMap.push({ node: tNode, offset: o });
                fullText += val[o];
              }
            }

            var cleanTarget = targetText.replace(/\\s+/g, ' ');
            var normFull = '';
            var normToOrigMap = [];
            var inSpace = false;
            for (var c = 0; c < fullText.length; c++) {
              var ch = fullText[c];
              if (/\\s/.test(ch)) {
                if (!inSpace) {
                  normToOrigMap.push(c);
                  normFull += ' ';
                  inSpace = true;
                }
              } else {
                normToOrigMap.push(c);
                normFull += ch;
                inSpace = false;
              }
            }

            var foundIdx = normFull.indexOf(cleanTarget);
            if (foundIdx === -1) {
              foundIdx = normFull.toLowerCase().indexOf(cleanTarget.toLowerCase());
            }

            if (foundIdx !== -1) {
              var origStart = normToOrigMap[foundIdx];
              var origEndIdx = Math.min(foundIdx + cleanTarget.length - 1, normToOrigMap.length - 1);
              var origEnd = normToOrigMap[origEndIdx] + 1;
              wrapAcrossNodes(nodeMap, origStart, origEnd, h.id, h.color || 'yellow');
            }
          }

          function restoreHighlights(list) {
            if (!list || !list.length) return;
            existingHighlights = list;
            for (var i = 0; i < list.length; i++) {
              var h = list[i];
              if (!h || !h.text) continue;
              if (document.querySelector('mark[data-highlight-id="' + h.id + '"]')) continue;
              highlightTextInDocument(h);
            }
          }

          function scrollToText(text) {
            if (!text) return false;
            var cleanTarget = text.trim().replace(/\\s+/g, ' ').toLowerCase();
            var sample = cleanTarget.substring(0, Math.min(32, cleanTarget.length));
            var nodes = getAllTextNodes(document.body);
            for (var i = 0; i < nodes.length; i++) {
              var val = nodes[i].nodeValue.replace(/\\s+/g, ' ').toLowerCase();
              if (val.indexOf(sample) !== -1) {
                var el = nodes[i].parentElement || nodes[i];
                el.scrollIntoView({ behavior: 'smooth', block: 'center' });
                el.classList.remove('reader-target-highlight');
                void el.offsetWidth;
                el.classList.add('reader-target-highlight');
                setTimeout(function() {
                  el.classList.remove('reader-target-highlight');
                }, 2000);
                return true;
              }
            }
            return false;
          }

          function jumpToHighlightWithRetry(highlightId, targetText, retries) {
            var mark = document.querySelector('mark[data-highlight-id="' + highlightId + '"]');
            if (mark) {
              mark.scrollIntoView({ behavior: 'smooth', block: 'center' });
              mark.classList.remove('reader-target-highlight');
              void mark.offsetWidth;
              mark.classList.add('reader-target-highlight');
              setTimeout(function() {
                mark.classList.remove('reader-target-highlight');
              }, 2000);
              return;
            }

            if (targetText && scrollToText(targetText)) {
              return;
            }

            if ((retries || 0) < 6) {
              setTimeout(function() {
                jumpToHighlightWithRetry(highlightId, targetText, (retries || 0) + 1);
              }, 120);
            }
          }

          function scrollToOffsetWithRetry(offset, retries) {
            var target = Math.max(0, offset || 0);
            window.scrollTo({ top: target, behavior: 'smooth' });
            if (document.documentElement) document.documentElement.scrollTop = target;
            if (document.body) document.body.scrollTop = target;
            if ((retries || 0) < 6) {
              setTimeout(function() {
                var curr = window.scrollY || (document.documentElement ? document.documentElement.scrollTop : 0) || (document.body ? document.body.scrollTop : 0) || 0;
                if (target > 10 && Math.abs(curr - target) > 30) {
                  scrollToOffsetWithRetry(target, (retries || 0) + 1);
                }
              }, 120);
            }
          }

          function setupContentEnhancements() {
            try {
              // 1. Clean up empty blockquotes
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

              // 2. Collapse artificial spacer divs/p elements
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

              // 3. Detect Table of Contents pages
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

              // 4. Intercept Footnotes for In-Place Popovers
              document.addEventListener('click', function(e) {
                var a = e.target.closest('a');
                if (!a) return;
                var href = a.getAttribute('href') || '';
                var hashIdx = href.indexOf('#');
                if (hashIdx === -1) return;
                var hash = href.substring(hashIdx + 1);
                if (!hash) return;

                var target = document.getElementById(hash) || document.querySelector('[name="' + CSS.escape(hash) + '"]');
                if (!target) return;

                var noteBlock = target.closest('li, p, div.calibre1, [class*="footnote" i], [class*="endnote" i]') || target;

                if (isFootnoteLink(a, href)) {
                  e.preventDefault();
                  e.stopPropagation();
                  showFootnotePopover(a, noteBlock);
                } else {
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

              // Dismiss popover on click outside or scroll
              document.addEventListener('mousedown', function(e) {
                if (activeFootnotePopover && !activeFootnotePopover.contains(e.target) && !e.target.closest('a')) {
                  removeFootnotePopover();
                }
                if (activeNoteBox && !activeNoteBox.contains(e.target)) {
                  removeNoteBox();
                }
              });

              window.addEventListener('scroll', function() {
                if (activeFootnotePopover) removeFootnotePopover();
                if (activeMenu) removeMenu();
                if (activeNoteBox) removeNoteBox();
              }, { passive: true });

              // Keyboard shortcuts inside iframe
              window.addEventListener('keydown', function(e) {
                if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
                  e.preventDefault();
                  window.parent.postMessage({ type: 'open-search-shortcut' }, '*');
                } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'd') {
                  e.preventDefault();
                  window.parent.postMessage({ type: 'toggle-bookmark-shortcut' }, '*');
                }
              });

              // Setup text selection toolbar
              setupSelectionToolbar();

              // Tell parent we are ready to receive highlights
              window.parent.postMessage({ type: 'request-highlights' }, '*');

            } catch (err) {
              console.warn('content enhancements error:', err);
            }
          }

          function wrapRangeWithMark(range, id, color) {
            try {
              if (range.startContainer === range.endContainer && range.startContainer.nodeType === Node.TEXT_NODE) {
                var mark = document.createElement('mark');
                mark.className = 'reader-highlight reader-highlight-' + color;
                mark.setAttribute('data-highlight-id', id);
                mark.setAttribute('data-color', color);
                var frag = range.extractContents();
                mark.appendChild(frag);
                range.insertNode(mark);
                return true;
              }

              var commonAncestor = range.commonAncestorContainer;
              var walker = document.createTreeWalker(
                commonAncestor.nodeType === Node.TEXT_NODE ? commonAncestor.parentElement : commonAncestor,
                NodeFilter.SHOW_TEXT,
                null,
                false
              );
              var textNodes = [];
              var curr;
              while (curr = walker.nextNode()) {
                if (range.intersectsNode(curr)) {
                  textNodes.push(curr);
                }
              }

              if (!textNodes.length) return false;

              for (var i = textNodes.length - 1; i >= 0; i--) {
                var node = textNodes[i];
                var start = (node === range.startContainer) ? range.startOffset : 0;
                var end = (node === range.endContainer) ? range.endOffset : node.nodeValue.length;
                if (start < end) {
                  wrapTextNodeRange(node, start, end, id, color);
                }
              }
              return true;
            } catch (err) {
              console.warn('wrapRangeWithMark error:', err);
              return false;
            }
          }

          function setupSelectionToolbar() {
            var colorList = [
              { id: 'yellow', hex: '#eab308' },
              { id: 'green',  hex: '#22c55e' },
              { id: 'blue',   hex: '#3b82f6' },
              { id: 'purple', hex: '#a855f7' },
              { id: 'pink',   hex: '#f43f5e' }
            ];

            function handleSelectionEnd() {
              var sel = window.getSelection();
              if (!sel || sel.isCollapsed) return;
              var selectedText = sel.toString().trim();
              if (!selectedText) return;

              var range = sel.getRangeAt(0);
              var rect = range.getBoundingClientRect();
              if (rect.width === 0 && rect.height === 0) return;

              removeMenu();
              removeFootnotePopover();

              var menu = document.createElement('div');
              menu.className = 'reader-selection-menu';

              colorList.forEach(function(c) {
                var btn = document.createElement('button');
                btn.className = 'reader-color-btn';
                btn.style.background = c.hex;
                btn.title = 'Highlight ' + c.id;
                btn.addEventListener('mousedown', function(e) {
                  e.preventDefault();
                  e.stopPropagation();

                  createHighlightFromRange(range, selectedText, c.id);
                  sel.removeAllRanges();
                  removeMenu();
                });
                menu.appendChild(btn);
              });

              var div = document.createElement('div');
              div.className = 'reader-menu-divider';
              menu.appendChild(div);

              var defBtn = document.createElement('button');
              defBtn.className = 'reader-icon-btn';
              defBtn.title = 'Define Word';
              defBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>';
              defBtn.addEventListener('mousedown', function(e) {
                e.preventDefault();
                e.stopPropagation();

                var firstWord = selectedText.trim().split(/\\s+/)[0];
                window.parent.postMessage({
                  type: 'open-dictionary',
                  word: firstWord,
                  x: rect.left + (rect.width / 2),
                  y: rect.top
                }, '*');
                sel.removeAllRanges();
                removeMenu();
              });
              menu.appendChild(defBtn);

              document.body.appendChild(menu);
              activeMenu = menu;

              var menuWidth = 196;
              var top = rect.top + window.scrollY - 44;
              if (top < window.scrollY + 10) {
                top = rect.bottom + window.scrollY + 8;
              }
              var left = rect.left + window.scrollX + (rect.width / 2) - (menuWidth / 2);
              left = Math.max(12, Math.min(left, document.body.clientWidth - menuWidth - 12));

              menu.style.top = top + 'px';
              menu.style.left = left + 'px';
            }

            document.addEventListener('mouseup', function(e) {
              if (e.target.closest('.reader-selection-menu') || e.target.closest('.reader-inline-note-box')) return;
              setTimeout(handleSelectionEnd, 30);
            });

            document.addEventListener('mousedown', function(e) {
              if (activeMenu && !activeMenu.contains(e.target) && !e.target.closest('mark.reader-highlight')) {
                removeMenu();
              }
            });

            // Handle clicking existing mark
            document.addEventListener('click', function(e) {
              var mark = e.target.closest('mark.reader-highlight');
              if (!mark) return;

              e.preventDefault();
              e.stopPropagation();

              var highlightId = mark.getAttribute('data-highlight-id');
              var currentColor = mark.getAttribute('data-color') || 'yellow';
              var rect = mark.getBoundingClientRect();

              removeMenu();
              removeFootnotePopover();
              removeNoteBox();

              var menu = document.createElement('div');
              menu.className = 'reader-selection-menu';

              colorList.forEach(function(c) {
                var btn = document.createElement('button');
                btn.className = 'reader-color-btn';
                btn.style.background = c.hex;
                if (c.id === currentColor) {
                  btn.style.borderColor = '#ffffff';
                }
                btn.title = 'Change to ' + c.id;
                btn.addEventListener('mousedown', function(ev) {
                  ev.preventDefault();
                  ev.stopPropagation();

                  var allMarks = document.querySelectorAll('mark[data-highlight-id="' + highlightId + '"]');
                  for (var m = 0; m < allMarks.length; m++) {
                    allMarks[m].className = 'reader-highlight reader-highlight-' + c.id;
                    allMarks[m].setAttribute('data-color', c.id);
                  }

                  window.parent.postMessage({
                    type: 'update-highlight',
                    highlightId: highlightId,
                    color: c.id
                  }, '*');
                  removeMenu();
                });
                menu.appendChild(btn);
              });

              var divider1 = document.createElement('div');
              divider1.className = 'reader-menu-divider';
              menu.appendChild(divider1);

              // Note button
              var noteBtn = document.createElement('button');
              noteBtn.className = 'reader-icon-btn';
              noteBtn.title = 'Add note';
              noteBtn.innerHTML = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg>';
              noteBtn.addEventListener('mousedown', function(ev) {
                ev.preventDefault();
                ev.stopPropagation();

                showNoteEditor(mark, highlightId);
                removeMenu();
              });
              menu.appendChild(noteBtn);

              var divider2 = document.createElement('div');
              divider2.className = 'reader-menu-divider';
              menu.appendChild(divider2);

              var markDefBtn = document.createElement('button');
              markDefBtn.className = 'reader-icon-btn';
              markDefBtn.title = 'Define Word';
              markDefBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>';
              markDefBtn.addEventListener('mousedown', function(ev) {
                ev.preventDefault();
                ev.stopPropagation();

                var firstWord = (mark.textContent || '').trim().split(/\\s+/)[0];
                window.parent.postMessage({
                  type: 'open-dictionary',
                  word: firstWord,
                  x: rect.left + (rect.width / 2),
                  y: rect.top
                }, '*');
                removeMenu();
              });
              menu.appendChild(markDefBtn);

              var divider3 = document.createElement('div');
              divider3.className = 'reader-menu-divider';
              menu.appendChild(divider3);

              var delBtn = document.createElement('button');
              delBtn.className = 'reader-delete-btn';
              delBtn.title = 'Delete highlight';
              delBtn.innerHTML = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>';
              delBtn.addEventListener('mousedown', function(ev) {
                ev.preventDefault();
                ev.stopPropagation();

                var allMarks = document.querySelectorAll('mark[data-highlight-id="' + highlightId + '"]');
                for (var m = 0; m < allMarks.length; m++) {
                  var mEl = allMarks[m];
                  var parent = mEl.parentNode;
                  while (mEl.firstChild) {
                    parent.insertBefore(mEl.firstChild, mEl);
                  }
                  parent.removeChild(mEl);
                }

                window.parent.postMessage({
                  type: 'delete-highlight',
                  highlightId: highlightId
                }, '*');
                removeMenu();
              });
              menu.appendChild(delBtn);

              document.body.appendChild(menu);
              activeMenu = menu;

              var menuWidth = 230;
              var top = rect.top + window.scrollY - 44;
              if (top < window.scrollY + 10) {
                top = rect.bottom + window.scrollY + 8;
              }
              var left = rect.left + window.scrollX + (rect.width / 2) - (menuWidth / 2);
              left = Math.max(12, Math.min(left, document.body.clientWidth - menuWidth - 12));

              menu.style.top = top + 'px';
              menu.style.left = left + 'px';
            });
          }

          function showNoteEditor(mark, highlightId) {
            removeNoteBox();

            var currentNote = '';
            for (var i = 0; i < existingHighlights.length; i++) {
              if (existingHighlights[i].id === highlightId) {
                currentNote = existingHighlights[i].note || '';
                break;
              }
            }

            var rect = mark.getBoundingClientRect();
            var box = document.createElement('div');
            box.className = 'reader-inline-note-box';
            box.innerHTML = [
              '<textarea rows="3" placeholder="Add a note...">' + (currentNote || '') + '</textarea>',
              '<div class="note-btn-row">',
              '  <button class="btn-cancel-note">Cancel</button>',
              '  <button class="btn-save-note">Save</button>',
              '</div>'
            ].join('');

            box.querySelector('.btn-cancel-note').addEventListener('click', function(e) {
              e.stopPropagation();
              removeNoteBox();
            });

            box.querySelector('.btn-save-note').addEventListener('click', function(e) {
              e.stopPropagation();
              var txt = box.querySelector('textarea').value.trim();
              for (var i = 0; i < existingHighlights.length; i++) {
                if (existingHighlights[i].id === highlightId) {
                  existingHighlights[i].note = txt;
                  break;
                }
              }
              window.parent.postMessage({
                type: 'save-highlight-note',
                highlightId: highlightId,
                note: txt
              }, '*');
              removeNoteBox();
            });

            document.body.appendChild(box);
            activeNoteBox = box;

            var top = rect.bottom + window.scrollY + 8;
            var left = rect.left + window.scrollX;
            left = Math.max(12, Math.min(left, document.body.clientWidth - 272));

            box.style.top = top + 'px';
            box.style.left = left + 'px';
            box.querySelector('textarea').focus();
          }

          function createHighlightFromRange(range, text, color) {
            var id = 'hl-' + Date.now() + '-' + Math.random().toString(36).substr(2, 6);

            var wrapped = wrapRangeWithMark(range, id, color);
            if (!wrapped) {
              highlightTextInDocument({ id: id, text: text, color: color });
            }

            var highlightData = {
              id: id,
              bookId: bookId,
              spineIndex: spineIndex,
              text: text,
              color: color,
              note: '',
              createdAt: new Date().toISOString()
            };

            existingHighlights.push(highlightData);

            window.parent.postMessage({
              type: 'create-highlight',
              highlight: highlightData
            }, '*');
          }

          function findHashTarget(rawHash) {
            if (!rawHash) return null;
            rawHash = String(rawHash).trim().replace(/^#+/, '');
            if (!rawHash) return null;

            var candidates = [rawHash];
            try {
              var decoded = decodeURIComponent(rawHash);
              if (decoded && candidates.indexOf(decoded) === -1) candidates.push(decoded);
            } catch(err) {}
            try {
              var unescaped = unescape(rawHash);
              if (unescaped && candidates.indexOf(unescaped) === -1) candidates.push(unescaped);
            } catch(err) {}

            for (var i = 0; i < candidates.length; i++) {
              var h = candidates[i];
              var el = document.getElementById(h);
              if (el) return el;

              var named = document.getElementsByName(h);
              if (named && named.length > 0) return named[0];

              try {
                el = document.querySelector('[name="' + CSS.escape(h) + '"]');
                if (el) return el;
              } catch(err) {}

              try {
                el = document.querySelector('#' + CSS.escape(h));
                if (el) return el;
              } catch(err) {}

              try {
                el = document.querySelector('[id*="' + CSS.escape(h) + '"]') || document.querySelector('[name*="' + CSS.escape(h) + '"]');
                if (el) return el;
              } catch(err) {}
            }

            var lowerCandidates = candidates.map(function(c) { return c.toLowerCase(); });
            var allAnchors = document.querySelectorAll('[id], [name]');
            for (var j = 0; j < allAnchors.length; j++) {
              var cEl = allAnchors[j];
              var cId = (cEl.id || '').toLowerCase();
              var cName = (cEl.getAttribute('name') || '').toLowerCase();
              for (var k = 0; k < lowerCandidates.length; k++) {
                if (cId === lowerCandidates[k] || cName === lowerCandidates[k]) {
                  return cEl;
                }
              }
            }

            return null;
          }

          function scrollElementIntoView(target) {
            var rect = target.getBoundingClientRect();
            var currentScroll = window.pageYOffset || (document.documentElement ? document.documentElement.scrollTop : 0) || (document.body ? document.body.scrollTop : 0) || 0;

            var scrollElem = target;
            if (scrollElem.offsetHeight === 0 && scrollElem.nextElementSibling) {
              var nextRect = scrollElem.nextElementSibling.getBoundingClientRect();
              if (nextRect.height > 0) {
                rect = nextRect;
                scrollElem = scrollElem.nextElementSibling;
              }
            }

            var targetY = Math.max(0, Math.round(currentScroll + rect.top - 24));
            window.scrollTo(0, targetY);
            if (document.documentElement) document.documentElement.scrollTop = targetY;
            if (document.body) document.body.scrollTop = targetY;

            target.classList.remove('reader-target-highlight');
            void target.offsetWidth;
            target.classList.add('reader-target-highlight');
            setTimeout(function() {
              target.classList.remove('reader-target-highlight');
            }, 1800);
          }

          function scrollToHashWithRetry(hash, attempts) {
            attempts = attempts || 0;
            var target = findHashTarget(hash);
            if (target) {
              window.parent.postMessage({ type: '_debug', msg: '[IFRAME] scrollToHash found: ' + hash + ' tag=' + target.tagName + ' id=' + target.id + ' attempt=' + attempts }, '*');
              scrollElementIntoView(target);
              setTimeout(function() {
                var t = findHashTarget(hash);
                if (t) {
                  var r = t.getBoundingClientRect();
                  if (Math.abs(r.top - 24) > 15) {
                    scrollElementIntoView(t);
                  }
                }
              }, 200);
              return;
            }
            if (attempts < 15) {
              if (attempts === 0) {
                window.parent.postMessage({ type: '_debug', msg: '[IFRAME] scrollToHash NOT found yet: ' + hash + ', retrying...' }, '*');
              }
              setTimeout(function() {
                scrollToHashWithRetry(hash, attempts + 1);
              }, 80);
            } else {
              window.parent.postMessage({ type: '_debug', msg: '[IFRAME] scrollToHash FAILED after 15 attempts: ' + hash }, '*');
            }
          }

          // Listen for commands from parent window
          window.addEventListener('message', function(e) {
            if (!e.data || !e.data.type) return;

            if (e.data.type === 'sync-highlights') {
              restoreHighlights(e.data.highlights || []);
            } else if (e.data.type === 'jump-to-highlight') {
              jumpToHighlightWithRetry(e.data.highlightId, e.data.text, 0);
            } else if (e.data.type === 'scroll-to-offset') {
              scrollToOffsetWithRetry(e.data.offset, 0);
            } else if (e.data.type === 'scroll-to-hash') {
              if (e.data.hash) {
                scrollToHashWithRetry(e.data.hash, 0);
              }
            } else if (e.data.type === 'scroll-to-top') {
              window.scrollTo(0, 0);
              if (document.documentElement) document.documentElement.scrollTop = 0;
              if (document.body) document.body.scrollTop = 0;
            } else if (e.data.type === 'remove-highlight-mark') {
              var allMarks = document.querySelectorAll('mark[data-highlight-id="' + e.data.highlightId + '"]');
              for (var m = 0; m < allMarks.length; m++) {
                var mEl = allMarks[m];
                var parent = mEl.parentNode;
                while (mEl.firstChild) {
                  parent.insertBefore(mEl.firstChild, mEl);
                }
                parent.removeChild(mEl);
              }
            } else if (e.data.type === 'find-and-scroll') {
              var query = (e.data.query || '').trim().toLowerCase();
              if (!query) return;

              var walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, null, false);
              var node;
              while (node = walker.nextNode()) {
                if (node.parentElement && node.parentElement.tagName === 'SCRIPT') continue;
                var val = node.nodeValue || '';
                var idx = val.toLowerCase().indexOf(query);
                if (idx !== -1) {
                  var span = document.createElement('span');
                  span.className = 'reader-search-match';
                  var range = document.createRange();
                  range.setStart(node, idx);
                  range.setEnd(node, idx + query.length);
                  var frag = range.extractContents();
                  span.appendChild(frag);
                  range.insertNode(span);

                  span.scrollIntoView({ behavior: 'smooth', block: 'center' });

                  setTimeout(function() {
                    if (span.parentNode) {
                      var p = span.parentNode;
                      while (span.firstChild) {
                        p.insertBefore(span.firstChild, span);
                      }
                      p.removeChild(span);
                    }
                  }, 4000);
                  break;
                }
              }
            }
          });

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

  function handleKeydown(e) {
    if (e.key === 'ArrowRight' || e.key === 'PageDown') {
      e.preventDefault();
      navigate(1);
    } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
      e.preventDefault();
      navigate(-1);
    } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
      e.preventDefault();
      searchOpen.set(true);
    } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'd') {
      e.preventDefault();
      toggleCurrentBookmark();
    }
  }

  function handleWindowMessage(e) {
    if (!e.data || !e.data.type) return;

    if (e.data.type === 'iframe-ready') {
      console.log(`[IFRAME_READY] navGen=${e.data.navGen}, spineIndex=${e.data.spineIndex}, currentSpineIndex=${$currentSpineIndex}, pendingHash=${pendingHash}`);
      
      if (e.data.navGen !== handledNavigationGeneration) {
        handledNavigationGeneration = e.data.navGen;
        if (pendingHash) {
          const h = pendingHash;
          pendingHash = null;
          isNavigatingToStart = false;
          console.log(`[IFRAME_READY] scrolling to hash: ${h}`);
          setTimeout(() => {
            iframeEl?.contentWindow?.postMessage({ type: 'scroll-to-hash', hash: h }, '*');
            setTimeout(queuePageCalculation, 120);
          }, 50);
        } else if (isNavigatingToStart) {
          isNavigatingToStart = false;
          console.log('[IFRAME_READY] scrolling to top (new chapter)');
          iframeEl?.contentWindow?.postMessage({ type: 'scroll-to-top' }, '*');
          setTimeout(queuePageCalculation, 60);
        } else {
          console.log('[IFRAME_READY] restoring saved progress');
          getProgress($currentBookId).then((pos) => {
            if (pos && pos.spineIndex === $currentSpineIndex && pos.scrollOffset && iframeEl?.contentWindow) {
              iframeEl.contentWindow.postMessage({ type: 'scroll-to-offset', offset: pos.scrollOffset }, '*');
            } else if (iframeEl?.contentWindow) {
              iframeEl.contentWindow.postMessage({ type: 'scroll-to-top' }, '*');
            }
            setTimeout(queuePageCalculation, 100);
          });
        }
      } else {
        // Theme change or reload: just restore progress
        console.log('[IFRAME_READY] generation already handled, restoring saved progress');
        getProgress($currentBookId).then((pos) => {
          if (pos && pos.spineIndex === $currentSpineIndex && pos.scrollOffset && iframeEl?.contentWindow) {
            iframeEl.contentWindow.postMessage({ type: 'scroll-to-offset', offset: pos.scrollOffset }, '*');
          } else if (iframeEl?.contentWindow) {
            iframeEl.contentWindow.postMessage({ type: 'scroll-to-top' }, '*');
          }
          setTimeout(queuePageCalculation, 100);
        });
      }

      if ($currentBookId) {
        const chHighlights = $highlights.filter(h => h.bookId === $currentBookId && h.spineIndex === $currentSpineIndex);
        iframeEl?.contentWindow?.postMessage({
          type: 'sync-highlights',
          highlights: chHighlights
        }, '*');
      }
      return;
    }

    if (e.data.type === '_debug') {
      console.log(e.data.msg);
      return;
    }

    if (e.data.type === 'create-highlight') {
      const h = e.data.highlight;
      addHighlight(h);
      highlights.update(items => [...items, h]);
    } else if (e.data.type === 'update-highlight') {
      const { highlightId, color } = e.data;
      highlights.update(items => items.map(h => {
        if (h.id === highlightId) {
          const updated = { ...h, color };
          addHighlight(updated);
          return updated;
        }
        return h;
      }));
    } else if (e.data.type === 'delete-highlight') {
      const { highlightId } = e.data;
      deleteHighlight(highlightId);
      highlights.update(items => items.filter(h => h.id !== highlightId));
    } else if (e.data.type === 'save-highlight-note') {
      const { highlightId, note } = e.data;
      updateHighlightNote(highlightId, note);
      highlights.update(items => items.map(h => h.id === highlightId ? { ...h, note } : h));
    } else if (e.data.type === 'request-highlights') {
      if (iframeEl?.contentWindow && $currentBookId) {
        const chHighlights = $highlights.filter(h => h.bookId === $currentBookId && h.spineIndex === $currentSpineIndex);
        iframeEl.contentWindow.postMessage({
          type: 'sync-highlights',
          highlights: chHighlights
        }, '*');
      }
    } else if (e.data.type === 'open-search-shortcut') {
      searchOpen.set(true);
    } else if (e.data.type === 'toggle-bookmark-shortcut') {
      toggleCurrentBookmark();
    } else if (e.data.type === 'open-dictionary') {
      const { word, x, y } = e.data;
      const iframeRect = iframeEl ? iframeEl.getBoundingClientRect() : { left: 0, top: 0 };
      activeDictionary = {
        word,
        x: Math.round(iframeRect.left + x),
        y: Math.round(iframeRect.top + y),
        placement: 'top'
      };
    }
  }

  let pendingHash = null;
  let isNavigatingToStart = false;

  let _navId = 0;

  async function handleNavigateTo(e) {
    const { spineIndex, hash } = e.detail || {};
    const navId = ++_navId;
    console.log(`[NAV ${navId}] handleNavigateTo called: spineIndex=${spineIndex}, hash=${hash}, currentSpine=${$currentSpineIndex}, spineCount=${$spineCount}, loading=${loading}`);

    if (typeof spineIndex !== 'number' || isNaN(spineIndex)) {
      console.log(`[NAV ${navId}] REJECTED: spineIndex not a number`);
      return;
    }
    if (spineIndex < 0 || ($spineCount > 0 && spineIndex >= $spineCount)) {
      console.log(`[NAV ${navId}] REJECTED: out of range (spineIndex=${spineIndex}, spineCount=${$spineCount})`);
      return;
    }

    if ($currentSpineIndex === spineIndex) {
      console.log(`[NAV ${navId}] SAME SPINE path: sending postMessage (hash=${hash}, hasIframe=${!!iframeEl?.contentWindow})`);
      if (hash) {
        if (iframeEl?.contentWindow) {
          iframeEl.contentWindow.postMessage({ type: 'scroll-to-hash', hash }, '*');
        } else {
          console.warn(`[NAV ${navId}] NO IFRAME contentWindow for hash scroll!`);
        }
      } else {
        if (iframeEl?.contentWindow) {
          iframeEl.contentWindow.postMessage({ type: 'scroll-to-top' }, '*');
        }
        if ($currentBookId) {
          saveProgress($currentBookId, spineIndex, 0);
        }
      }
      return;
    }

    loading = true;
    console.log(`[NAV ${navId}] DIFFERENT SPINE: fetching chapter ${spineIndex}`);

    try {
      const html = await getChapter($currentBookId, spineIndex);
      console.log(`[NAV ${navId}] getChapter returned, html length=${html?.length}, setting stores. _navId is now ${_navId}`);
      if (navId !== _navId) {
        console.warn(`[NAV ${navId}] STALE: a newer nav (${_navId}) was started, skipping store update`);
        return;
      }
      
      pendingHash = hash || null;
      isNavigatingToStart = !hash;
      navigationGeneration++;
      
      currentSpineIndex.set(spineIndex);
      currentChapter.set(html);
      if (!hash && $currentBookId) {
        saveProgress($currentBookId, spineIndex, 0);
      }
    } catch (err) {
      console.error(`[NAV ${navId}] Failed to load chapter:`, err);
    } finally {
      loading = false;
    }
  }

  function handleScrollToHash(e) {
    const hash = e.detail?.hash;
    if (!hash) return;
    pendingHash = hash;
    if (iframeEl?.contentWindow) {
      iframeEl.contentWindow.postMessage({ type: 'scroll-to-hash', hash }, '*');
    }
  }

  function handleScrollToTop() {
    if (iframeEl?.contentWindow) {
      iframeEl.contentWindow.scrollTo({ top: 0, behavior: 'smooth' });
      iframeEl.contentWindow.postMessage({ type: 'scroll-to-top' }, '*');
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('message', handleWindowMessage);
    window.addEventListener('epub-scroll-to-hash', handleScrollToHash);
    window.addEventListener('epub-scroll-to-top', handleScrollToTop);
    window.addEventListener('epub-navigate-to', handleNavigateTo);
    return () => {
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('message', handleWindowMessage);
      window.removeEventListener('epub-scroll-to-hash', handleScrollToHash);
      window.removeEventListener('epub-scroll-to-top', handleScrollToTop);
      window.removeEventListener('epub-navigate-to', handleNavigateTo);
    };
  });

  let saveProgressTimer = null;
  let scrollDebounceTimer = null;
  let iframeResizeObserver = null;
  let pageCalcRAF = null;

  function getChapterTitle(spineIndex) {
    function findTitle(entries) {
      if (!entries) return null;
      for (const e of entries) {
        if (e.spineIndex === spineIndex) return e.title;
        if (e.children && e.children.length > 0) {
          const t = findTitle(e.children);
          if (t) return t;
        }
      }
      return null;
    }
    return findTitle($toc) || `Chapter ${spineIndex + 1}`;
  }

  function updateChapterPages() {
    if (!iframeEl?.contentWindow) return;
    const win = iframeEl.contentWindow;
    const doc = win.document;
    if (!doc || !doc.documentElement) return;

    const clientHeight = iframeEl.clientHeight || win.innerHeight || 800;
    const scrollHeight = Math.max(
      doc.documentElement.scrollHeight || 0,
      doc.body ? doc.body.scrollHeight : 0
    );
    const scrollTop = win.scrollY || doc.documentElement.scrollTop || 0;

    // Viewport height defines one screen page
    const totalPages = Math.max(1, Math.ceil(scrollHeight / clientHeight));
    
    // Check if user is scrolled to the very bottom
    const isAtBottom = (scrollTop + clientHeight) >= (scrollHeight - 15);
    const currentPage = isAtBottom ? totalPages : Math.min(totalPages, Math.max(1, Math.floor(scrollTop / clientHeight) + 1));
    const pagesLeft = isAtBottom ? 0 : Math.max(0, totalPages - currentPage);

    const maxScroll = Math.max(1, scrollHeight - clientHeight);
    const percentInChapter = Math.min(100, Math.max(0, Math.round((scrollTop / maxScroll) * 100)));

    const title = getChapterTitle($currentSpineIndex);

    chapterPageInfo.set({
      currentPage,
      totalPages,
      pagesLeft,
      chapterTitle: title,
      percentInChapter: isAtBottom ? 100 : percentInChapter,
    });
  }

  function queuePageCalculation() {
    if (pageCalcRAF) cancelAnimationFrame(pageCalcRAF);
    pageCalcRAF = requestAnimationFrame(() => {
      updateChapterPages();
    });
  }

  // Recalculate if settings change
  $: if ($settings && iframeEl) {
    setTimeout(queuePageCalculation, 60);
  }

  function flushProgress() {
    if ($currentBookId && iframeEl?.contentWindow) {
      const scrollY = Math.round(iframeEl.contentWindow.scrollY || iframeEl.contentWindow.document?.documentElement?.scrollTop || 0);
      saveProgress($currentBookId, $currentSpineIndex, scrollY);
    }
  }

  function handleIframeLoad() {
    if (!iframeEl?.contentWindow) return;
    console.log(`[IFRAME_LOAD] native load event fired. (Handled via iframe-ready now)`);

    if (iframeResizeObserver) {
      iframeResizeObserver.disconnect();
      iframeResizeObserver = null;
    }

    try {
      iframeEl.contentWindow.addEventListener('scroll', () => {
        queuePageCalculation();
        if (scrollDebounceTimer) clearTimeout(scrollDebounceTimer);
        scrollDebounceTimer = setTimeout(() => {
          flushProgress();
        }, 500);
      }, { passive: true });

      iframeEl.contentWindow.addEventListener('resize', () => {
        queuePageCalculation();
      }, { passive: true });

      if (window.ResizeObserver && iframeEl.contentDocument?.body) {
        iframeResizeObserver = new ResizeObserver(() => {
          queuePageCalculation();
        });
        iframeResizeObserver.observe(iframeEl.contentDocument.body);
      }
    } catch (_) {}

    queuePageCalculation();
    setTimeout(queuePageCalculation, 150);
    setTimeout(queuePageCalculation, 450);

    if (saveProgressTimer) {
      clearInterval(saveProgressTimer);
    }
    saveProgressTimer = setInterval(() => {
      flushProgress();
    }, 4000);
  }

  onDestroy(() => {
    if (saveProgressTimer) {
      clearInterval(saveProgressTimer);
      saveProgressTimer = null;
    }
    if (scrollDebounceTimer) {
      clearTimeout(scrollDebounceTimer);
      scrollDebounceTimer = null;
    }
    if (iframeResizeObserver) {
      iframeResizeObserver.disconnect();
      iframeResizeObserver = null;
    }
    if (pageCalcRAF) {
      cancelAnimationFrame(pageCalcRAF);
      pageCalcRAF = null;
    }
    flushProgress();

    if (iframeEl) {
      try {
        iframeEl.srcdoc = '';
        iframeEl.src = 'about:blank';
      } catch (_) {}
    }
  });
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

  {#if activeDictionary}
    <DictionaryPopover
      word={activeDictionary.word}
      x={activeDictionary.x}
      y={activeDictionary.y}
      placement={activeDictionary.placement}
      on:close={() => { activeDictionary = null; }}
    />
  {/if}
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
    width: 24px;
    height: 24px;
    border: 2px solid var(--border-subtle);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .chapter-nav {
    position: absolute;
    bottom: 24px;
    right: 24px;
    display: flex;
    gap: 8px;
    z-index: 5;
    pointer-events: none;
  }

  .nav-btn {
    pointer-events: auto;
    width: 40px;
    height: 40px;
    border-radius: var(--radius-full);
    border: 1px solid var(--glass-border);
    background: var(--glass-bg);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    color: var(--fg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    box-shadow: var(--shadow-md);
    transition: transform var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out),
                background var(--duration-fast) var(--ease-out);
  }

  .nav-btn:hover:not(.disabled) {
    transform: scale(1.08);
    color: var(--fg-primary);
    background: var(--bg-hover);
  }

  .nav-btn:active:not(.disabled) {
    transform: scale(0.95);
  }

  .nav-btn.disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
</style>
