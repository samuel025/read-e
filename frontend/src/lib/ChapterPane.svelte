<script>
  import { onMount } from 'svelte';
  import {
    currentChapter, currentBookId, currentSpineIndex,
    spineCount, settings, highlights
  } from '../stores/app.js';
  import { getChapter, saveProgress, getProgress, addHighlight, deleteHighlight } from './api.js';

  let iframeEl;
  let loading = false;

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

  $: themeCSS = buildThemeCSS($settings);

  function buildThemeCSS(s) {
    const themes = {
      dark: { bg: '#0f1117', fg: '#e0e2e8', link: '#818cf8', highlight: 'rgba(129, 140, 248, 0.28)' },
      light: { bg: '#ffffff', fg: '#1a1d26', link: '#6366f1', highlight: 'rgba(99, 102, 241, 0.25)' },
      sepia: { bg: '#f5f0e8', fg: '#3d2e1c', link: '#a0522d', highlight: 'rgba(160, 82, 45, 0.22)' },
      nord: { bg: '#2e3440', fg: '#eceff4', link: '#88c0d0', highlight: 'rgba(136, 192, 208, 0.28)' },
    };
    const t = themes[s.theme] || themes.dark;
    const isDark = s.theme === 'dark' || s.theme === 'nord';

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

        /* ==================== MULTI-COLOR HIGHLIGHTING ==================== */
        mark.reader-highlight {
          border-radius: 3px;
          padding: 1px 3px;
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

        @media (prefers-reduced-motion: reduce) {
          * { transition: none !important; animation: none !important; }
        }

        [style*="font-family"] { font-family: inherit !important; }
      </style>
    `;
  }

  $: chapterHighlights = $highlights.filter(h => h.bookId === $currentBookId && h.spineIndex === $currentSpineIndex);
  $: srcdoc = buildSrcdoc($currentChapter, themeCSS, chapterHighlights, $currentBookId, $currentSpineIndex);

  function buildSrcdoc(chapter, css, highlightsList, bookId, spineIndex) {
    if (!chapter) return '';

    const script = `
      <script>
        (function() {
          var bookId = ${JSON.stringify(bookId || '')};
          var spineIndex = ${spineIndex || 0};
          var existingHighlights = ${JSON.stringify(highlightsList || [])};

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

              // 5. Restore saved highlights
              restoreHighlights(existingHighlights);

              // 6. Setup text selection toolbar
              setupSelectionToolbar();

            } catch (err) {
              console.warn('content enhancements error:', err);
            }
          }

          function restoreHighlights(list) {
            if (!list || !list.length) return;

            list.forEach(function(h) {
              if (document.querySelector('mark[data-highlight-id="' + h.id + '"]')) return;
              if (!h.text) return;

              wrapTextMatch(h);
            });
          }

          function wrapTextMatch(h) {
            var walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, null, false);
            var node;
            var candidates = [];
            while (node = walker.nextNode()) {
              if (node.parentElement && (node.parentElement.classList.contains('reader-highlight') || node.parentElement.closest('.reader-selection-menu'))) {
                continue;
              }
              var val = node.nodeValue;
              var pos = val.indexOf(h.text);
              if (pos !== -1) {
                candidates.push({ node: node, pos: pos });
              }
            }

            for (var c = 0; c < candidates.length; c++) {
              var cand = candidates[c];
              try {
                var range = document.createRange();
                range.setStart(cand.node, cand.pos);
                range.setEnd(cand.node, cand.pos + h.text.length);

                var mark = document.createElement('mark');
                mark.className = 'reader-highlight reader-highlight-' + (h.color || 'yellow');
                mark.setAttribute('data-highlight-id', h.id);
                mark.setAttribute('data-color', h.color || 'yellow');

                var frag = range.extractContents();
                mark.appendChild(frag);
                range.insertNode(mark);
                return;
              } catch (e) {}
            }
          }

          var activeMenu = null;

          function removeMenu() {
            if (activeMenu && activeMenu.parentNode) {
              activeMenu.parentNode.removeChild(activeMenu);
            }
            activeMenu = null;
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
              if (!sel || sel.isCollapsed) {
                return;
              }
              var selectedText = sel.toString().trim();
              if (!selectedText) {
                return;
              }

              var range = sel.getRangeAt(0);
              var rect = range.getBoundingClientRect();
              if (rect.width === 0 && rect.height === 0) return;

              removeMenu();

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

              document.body.appendChild(menu);
              activeMenu = menu;

              var menuWidth = 160;
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
              if (e.target.closest('.reader-selection-menu')) return;
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

                  mark.className = 'reader-highlight reader-highlight-' + c.id;
                  mark.setAttribute('data-color', c.id);
                  window.parent.postMessage({
                    type: 'update-highlight',
                    highlightId: highlightId,
                    color: c.id
                  }, '*');
                  removeMenu();
                });
                menu.appendChild(btn);
              });

              var divider = document.createElement('div');
              divider.className = 'reader-menu-divider';
              menu.appendChild(divider);

              var delBtn = document.createElement('button');
              delBtn.className = 'reader-delete-btn';
              delBtn.title = 'Delete highlight';
              delBtn.innerHTML = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>';
              delBtn.addEventListener('mousedown', function(ev) {
                ev.preventDefault();
                ev.stopPropagation();

                var parent = mark.parentNode;
                while (mark.firstChild) {
                  parent.insertBefore(mark.firstChild, mark);
                }
                parent.removeChild(mark);

                window.parent.postMessage({
                  type: 'delete-highlight',
                  highlightId: highlightId
                }, '*');
                removeMenu();
              });
              menu.appendChild(delBtn);

              document.body.appendChild(menu);
              activeMenu = menu;

              var menuWidth = 195;
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

          function createHighlightFromRange(range, text, color) {
            var id = 'hl-' + Date.now() + '-' + Math.random().toString(36).substr(2, 6);

            var mark = document.createElement('mark');
            mark.className = 'reader-highlight reader-highlight-' + color;
            mark.setAttribute('data-highlight-id', id);
            mark.setAttribute('data-color', color);

            try {
              var frag = range.extractContents();
              mark.appendChild(frag);
              range.insertNode(mark);
            } catch (e) {
              return;
            }

            var highlightData = {
              id: id,
              bookId: bookId,
              spineIndex: spineIndex,
              text: text,
              color: color,
              createdAt: new Date().toISOString()
            };

            window.parent.postMessage({
              type: 'create-highlight',
              highlight: highlightData
            }, '*');
          }

          // Listen for commands from parent window
          window.addEventListener('message', function(e) {
            if (!e.data || !e.data.type) return;

            if (e.data.type === 'jump-to-highlight') {
              var mark = document.querySelector('mark[data-highlight-id="' + e.data.highlightId + '"]');
              if (mark) {
                mark.scrollIntoView({ behavior: 'smooth', block: 'center' });
                mark.classList.remove('reader-target-highlight');
                void mark.offsetWidth;
                mark.classList.add('reader-target-highlight');
                setTimeout(function() {
                  mark.classList.remove('reader-target-highlight');
                }, 2000);
              }
            } else if (e.data.type === 'remove-highlight-mark') {
              var markToRemove = document.querySelector('mark[data-highlight-id="' + e.data.highlightId + '"]');
              if (markToRemove) {
                var parent = markToRemove.parentNode;
                while (markToRemove.firstChild) {
                  parent.insertBefore(markToRemove.firstChild, markToRemove);
                }
                parent.removeChild(markToRemove);
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
    }
  }

  function handleWindowMessage(e) {
    if (!e.data || !e.data.type) return;

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
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('message', handleWindowMessage);
    return () => {
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('message', handleWindowMessage);
    };
  });

  function handleIframeLoad() {
    if (!iframeEl?.contentWindow) return;

    getProgress($currentBookId).then((pos) => {
      if (pos?.scrollOffset && iframeEl?.contentWindow) {
        iframeEl.contentWindow.scrollTo(0, pos.scrollOffset);
      }
    });

    const interval = setInterval(() => {
      if (iframeEl?.contentWindow) {
        const scrollY = iframeEl.contentWindow.scrollY || 0;
        saveProgress($currentBookId, $currentSpineIndex, scrollY);
      }
    }, 5000);

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
