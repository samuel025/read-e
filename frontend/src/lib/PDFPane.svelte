<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import * as pdfjsLib from 'pdfjs-dist';
  import pdfjsWorker from 'pdfjs-dist/build/pdf.worker.min.js?url';
  import 'pdfjs-dist/web/pdf_viewer.css';

  import {
    currentBookId,
    currentBook,
    currentSpineIndex,
    toc,
    spineCount,
    readingStats,
    highlights,
    settings,
    pdfDoc,
    pdfZoom,
  } from '../stores/app.js';
  import {
    getPDFData,
    saveProgress,
    getProgress,
    addHighlight,
    deleteHighlight,
    updateHighlightNote,
  } from './api.js';
  import { getDocumentPageTexts } from './pdfSearch.js';

  pdfjsLib.GlobalWorkerOptions.workerSrc = pdfjsWorker;

  let viewportEl;
  let viewportWidth = 0;
  let loading = true;
  let error = null;
  let doc = null;
  let numPages = 0;
  let pageDimensions = [];
  let pageTops = [];
  let pageWordCounts = [];

  let renderedPages = new Map();
  let renderQueue = [];
  let renderTimeoutId = null;
  let currentlyRendering = false;
  let isScrolling = false;
  let scrollEndTimer = null;
  let scrollRAF = null;
  let saveProgressTimer = null;
  let resizeObserver = null;

  let selectionToolbar = {
    visible: false,
    x: 0,
    y: 0,
    pageIndex: 0,
    text: '',
    range: null,
  };

  let activeNotePopover = null;

  const colors = [
    { id: 'yellow', label: 'Yellow', hex: '#eab308' },
    { id: 'green', label: 'Green', hex: '#22c55e' },
    { id: 'blue', label: 'Blue', hex: '#3b82f6' },
    { id: 'purple', label: 'Purple', hex: '#a855f7' },
    { id: 'pink', label: 'Pink', hex: '#f43f5e' },
  ];

  function getScaleForPage(pageIndex) {
    const dims = pageDimensions[pageIndex];
    if (!dims) return 1.0;
    const baseWidth = dims.width || 612;
    const vWidth = viewportWidth || viewportEl?.clientWidth || 800;
    const availableWidth = Math.max(320, vWidth - 48);

    if ($pdfZoom === -1) {
      return Math.max(0.4, Math.min(3.0, availableWidth / baseWidth));
    }
    const fitScale = availableWidth / baseWidth;
    const pct = ($pdfZoom || 100) / 100;
    return Math.max(0.3, Math.min(3.5, fitScale * pct));
  }

  function computePageTops() {
    const tops = [];
    let currentY = 0;
    for (let i = 0; i < numPages; i++) {
      tops.push(currentY);
      const dims = pageDimensions[i] || { width: 612, height: 792 };
      const scale = getScaleForPage(i);
      const ph = Math.floor(dims.height * scale);
      currentY += ph + 24;
    }
    pageTops = tops;
  }

  onMount(async () => {
    if (viewportEl) {
      viewportWidth = viewportEl.clientWidth;
      resizeObserver = new ResizeObserver((entries) => {
        for (const entry of entries) {
          const newWidth = Math.floor(entry.contentRect.width);
          if (newWidth > 0 && Math.abs(newWidth - viewportWidth) > 6) {
            viewportWidth = newWidth;
            handleZoomChanged();
          }
        }
      });
      resizeObserver.observe(viewportEl);
    }

    await loadPDFDocument();

    window.addEventListener('resize', handleResize, { passive: true });
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('pdf-scroll-to-page', handleScrollToPageEvent);
    window.addEventListener('pdf-scroll-to-toc', handleScrollToTocEvent);
    window.addEventListener('pdf-scroll-to-bookmark', handleScrollToBookmarkEvent);
    window.addEventListener('pdf-jump-to-highlight', handleJumpToHighlightEvent);
    window.addEventListener('pdf-remove-highlight', handleRemoveHighlightEvent);
    window.addEventListener('pdf-find-and-scroll', handleFindAndScrollEvent);
    window.addEventListener('pdf-zoom-in', handleZoomIn);
    window.addEventListener('pdf-zoom-out', handleZoomOut);
    window.addEventListener('pdf-zoom-fit', handleZoomFit);

    document.addEventListener('selectionchange', handleSelectionChange);
  });

  onDestroy(() => {
    window.removeEventListener('resize', handleResize);
    window.removeEventListener('keydown', handleKeydown);
    window.removeEventListener('pdf-scroll-to-page', handleScrollToPageEvent);
    window.removeEventListener('pdf-scroll-to-toc', handleScrollToTocEvent);
    window.removeEventListener('pdf-scroll-to-bookmark', handleScrollToBookmarkEvent);
    window.removeEventListener('pdf-jump-to-highlight', handleJumpToHighlightEvent);
    window.removeEventListener('pdf-remove-highlight', handleRemoveHighlightEvent);
    window.removeEventListener('pdf-find-and-scroll', handleFindAndScrollEvent);
    window.removeEventListener('pdf-zoom-in', handleZoomIn);
    window.removeEventListener('pdf-zoom-out', handleZoomOut);
    window.removeEventListener('pdf-zoom-fit', handleZoomFit);
    document.removeEventListener('selectionchange', handleSelectionChange);

    if (scrollRAF) cancelAnimationFrame(scrollRAF);
    if (saveProgressTimer) clearTimeout(saveProgressTimer);
    if (scrollEndTimer) clearTimeout(scrollEndTimer);
    if (renderTimeoutId) clearTimeout(renderTimeoutId);

    if (resizeObserver) {
      resizeObserver.disconnect();
      resizeObserver = null;
    }

    cancelAllRenders();
    pdfDoc.set(null);
  });

  async function loadPDFDocument() {
    loading = true;
    error = null;
    const bookId = $currentBookId;
    if (!bookId) return;

    try {
      const data = await getPDFData(bookId);
      if (!data) throw new Error('No PDF data received from backend');

      const loadingTask = pdfjsLib.getDocument({
        data,
        cMapUrl: 'https://cdn.jsdelivr.net/npm/pdfjs-dist@3.11.174/cmaps/',
        cMapPacked: true,
      });

      doc = await loadingTask.promise;
      pdfDoc.set(doc);
      numPages = doc.numPages;
      spineCount.set(numPages);

      const dims = [];
      for (let i = 1; i <= numPages; i++) {
        const page = await doc.getPage(i);
        const vp = page.getViewport({ scale: 1.0 });
        dims.push({ width: vp.width, height: vp.height, aspectRatio: vp.height / vp.width });
        page.cleanup();
      }
      pageDimensions = dims;

      await extractOutline(doc);

      await tick();
      if (viewportEl) viewportWidth = viewportEl.clientWidth;

      computePageTops();
      calculateReadingStats(doc);

      loading = false;
      await tick();

      restoreSavedPosition(bookId);
    } catch (e) {
      console.error('Failed to load PDF document:', e);
      error = e.message || 'Unable to open PDF document';
      loading = false;
    }
  }

  async function extractOutline(pdf) {
    try {
      const rawOutline = await pdf.getOutline();
      if (rawOutline && rawOutline.length > 0) {
        const parsedTOC = await parseOutlineItems(rawOutline, pdf);
        toc.set(parsedTOC);
        return;
      }
    } catch (e) {}

    const fallbackTOC = [];
    const step = numPages > 60 ? 5 : 1;
    for (let i = 1; i <= numPages; i += step) {
      fallbackTOC.push({ title: `Page ${i}`, href: `#page-${i}`, spineIndex: i - 1 });
    }
    toc.set(fallbackTOC);
  }

  async function parseOutlineItems(items, pdf) {
    const result = [];
    for (const item of items) {
      let pageIndex = 0;
      try {
        let dest = item.dest;
        if (typeof dest === 'string') dest = await pdf.getDestination(dest);
        if (Array.isArray(dest) && dest[0]) pageIndex = await pdf.getPageIndex(dest[0]);
      } catch (_) {}

      const entry = { title: item.title, href: `#page-${pageIndex + 1}`, spineIndex: pageIndex };
      if (item.items && item.items.length > 0) {
        entry.children = await parseOutlineItems(item.items, pdf);
      }
      result.push(entry);
    }
    return result;
  }

  async function calculateReadingStats(pdf) {
    try {
      const pageTexts = await getDocumentPageTexts(pdf);
      const counts = [];
      let totalWords = 0;
      for (const text of pageTexts) {
        if (!text) { counts.push(0); continue; }
        const wCount = text.trim().split(/\s+/).filter(Boolean).length;
        counts.push(wCount);
        totalWords += wCount;
      }
      pageWordCounts = counts;
      updateMinutesRemaining($currentSpineIndex, totalWords);
    } catch (_) {}
  }

  function updateMinutesRemaining(activePageIndex, totalWordsFallback) {
    if (!pageWordCounts.length) {
      if (totalWordsFallback) {
        readingStats.set({ words: totalWordsFallback, minutesLeft: Math.max(1, Math.round(totalWordsFallback / 220)) });
      }
      return;
    }
    let wordsLeft = 0;
    for (let i = activePageIndex; i < pageWordCounts.length; i++) wordsLeft += pageWordCounts[i] || 0;
    const totalWords = pageWordCounts.reduce((a, b) => a + b, 0);
    readingStats.set({ words: totalWords, minutesLeft: Math.max(1, Math.round(wordsLeft / 220)) });
  }

  async function restoreSavedPosition(bookId) {
    try {
      const pos = await getProgress(bookId);
      if (pos && typeof pos.spineIndex === 'number' && pos.spineIndex >= 0) {
        currentSpineIndex.set(pos.spineIndex);
        await tick();
        scrollToPage(pos.spineIndex + 1, pos.scrollOffset || 0);
      } else {
        queueVisiblePages(0);
        scheduleRender();
      }
    } catch (_) {
      queueVisiblePages(0);
      scheduleRender();
    }
  }

  function getActivePageIndex() {
    if (!viewportEl || !pageTops.length) return 0;
    const currentScroll = viewportEl.scrollTop + 80;
    let low = 0, high = pageTops.length - 1, activeIndex = 0;
    while (low <= high) {
      const mid = (low + high) >> 1;
      if (pageTops[mid] <= currentScroll) {
        activeIndex = mid;
        low = mid + 1;
      } else {
        high = mid - 1;
      }
    }
    return activeIndex;
  }

  function handleScroll() {
    isScrolling = true;
    if (renderTimeoutId !== null) {
      clearTimeout(renderTimeoutId);
      renderTimeoutId = null;
    }

    clearTimeout(scrollEndTimer);
    scrollEndTimer = setTimeout(() => {
      isScrolling = false;
      const activeIdx = getActivePageIndex();
      queueVisiblePages(activeIdx);
      scheduleRender();
    }, 150);

    if (scrollRAF) return;
    scrollRAF = requestAnimationFrame(() => {
      scrollRAF = null;
      if (!viewportEl || !pageTops.length) return;

      const activeIndex = getActivePageIndex();
      if ($currentSpineIndex !== activeIndex) {
        currentSpineIndex.set(activeIndex);
        updateMinutesRemaining(activeIndex);
      }

      clearTimeout(saveProgressTimer);
      saveProgressTimer = setTimeout(() => {
        if ($currentBookId) saveProgress($currentBookId, activeIndex, viewportEl?.scrollTop || 0);
      }, 600);
    });
  }

  function queueVisiblePages(activeIdx) {
    const needed = [activeIdx];
    if (activeIdx + 1 < numPages) needed.push(activeIdx + 1);
    if (activeIdx - 1 >= 0) needed.push(activeIdx - 1);

    for (const [pIdx] of renderedPages) {
      if (Math.abs(pIdx - activeIdx) > 1) {
        unrenderPage(pIdx);
      }
    }

    for (const idx of needed) {
      if (!renderedPages.has(idx) && !renderQueue.includes(idx)) {
        renderQueue.push(idx);
      }
    }
  }

  function scheduleRender() {
    if (renderTimeoutId !== null) return;
    renderTimeoutId = setTimeout(processRenderQueue, 30);
  }

  async function processRenderQueue() {
    renderTimeoutId = null;
    if (isScrolling || currentlyRendering || !doc || !viewportEl) return;

    const activeIdx = getActivePageIndex();
    renderQueue = renderQueue.filter((idx) => Math.abs(idx - activeIdx) <= 1 && !renderedPages.has(idx));
    if (!renderQueue.length) return;

    renderQueue.sort((a, b) => Math.abs(a - activeIdx) - Math.abs(b - activeIdx));
    const pageIndex = renderQueue.shift();

    currentlyRendering = true;
    await renderPage(pageIndex);
    currentlyRendering = false;

    if (!isScrolling && renderQueue.length > 0) {
      renderTimeoutId = setTimeout(processRenderQueue, 60);
    }
  }

  async function renderPage(pageIndex) {
    if (!doc || renderedPages.has(pageIndex)) return;

    const pageContainer = document.getElementById(`page-${pageIndex + 1}`);
    if (!pageContainer) return;

    const pageNumber = pageIndex + 1;
    renderedPages.set(pageIndex, { loading: true });

    try {
      const page = await doc.getPage(pageNumber);
      const scale = getScaleForPage(pageIndex);
      const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
      const viewport = page.getViewport({ scale });

      const pw = Math.floor(viewport.width);
      const ph = Math.floor(viewport.height);

      pageContainer.style.width = `${pw}px`;
      pageContainer.style.height = `${ph}px`;

      const canvas = document.createElement('canvas');
      canvas.className = 'pdf-canvas';
      canvas.width = Math.floor(pw * dpr);
      canvas.height = Math.floor(ph * dpr);
      canvas.style.width = `${pw}px`;
      canvas.style.height = `${ph}px`;

      const ctx = canvas.getContext('2d', { alpha: false });
      const transform = dpr !== 1 ? [dpr, 0, 0, dpr, 0, 0] : null;

      const renderTask = page.render({ canvasContext: ctx, viewport, transform });
      renderedPages.set(pageIndex, { page, renderTask, canvas, scale });

      await renderTask.promise;

      if (!renderedPages.has(pageIndex)) {
        page.cleanup();
        return;
      }

      pageContainer.innerHTML = '';
      pageContainer.appendChild(canvas);

      const textLayerDiv = document.createElement('div');
      textLayerDiv.className = 'textLayer';
      textLayerDiv.style.width = `${pw}px`;
      textLayerDiv.style.height = `${ph}px`;
      textLayerDiv.style.setProperty('--scale-factor', viewport.scale);
      pageContainer.appendChild(textLayerDiv);

      renderedPages.set(pageIndex, { page, renderTask, canvas, textLayer: textLayerDiv, scale });

      page.getTextContent().then((textContent) => {
        if (!renderedPages.has(pageIndex)) return;
        const textLayerTask = pdfjsLib.renderTextLayer({
          textContentSource: textContent,
          container: textLayerDiv,
          viewport,
        });
        return textLayerTask.promise;
      }).then(() => {
        applyHighlightsToPage(pageIndex, textLayerDiv);
      }).catch(() => {});

      page.cleanup();
    } catch (err) {
      if (err.name !== 'RenderingCancelledException') {
        console.error(`Error rendering page ${pageNumber}:`, err);
      }
      renderedPages.delete(pageIndex);
    }
  }

  function unrenderPage(pageIndex) {
    const pageData = renderedPages.get(pageIndex);
    if (pageData) {
      if (pageData.renderTask) {
        try {
          pageData.renderTask.cancel();
        } catch (_) {}
      }
      renderedPages.delete(pageIndex);
    }

    const pageContainer = document.getElementById(`page-${pageIndex + 1}`);
    if (pageContainer) {
      const dims = pageDimensions[pageIndex];
      const scale = getScaleForPage(pageIndex);
      const ph = dims ? Math.floor(dims.height * scale) : 792;
      const pw = dims ? Math.floor(dims.width * scale) : 612;
      pageContainer.style.width = `${pw}px`;
      pageContainer.style.height = `${ph}px`;
      pageContainer.innerHTML = `<div class="pdf-page-placeholder" style="height: ${ph}px;"><span class="page-badge">Page ${pageIndex + 1}</span></div>`;
    }
  }

  function cancelAllRenders() {
    renderQueue = [];
    currentlyRendering = false;
    if (renderTimeoutId !== null) {
      clearTimeout(renderTimeoutId);
      renderTimeoutId = null;
    }
    renderedPages.forEach((data, pageIndex) => {
      if (data.renderTask) {
        try {
          data.renderTask.cancel();
        } catch (_) {}
      }
      const el = document.getElementById(`page-${pageIndex + 1}`);
      if (el) {
        const dims = pageDimensions[pageIndex];
        const scale = getScaleForPage(pageIndex);
        const ph = dims ? Math.floor(dims.height * scale) : 792;
        const pw = dims ? Math.floor(dims.width * scale) : 612;
        el.style.width = `${pw}px`;
        el.style.height = `${ph}px`;
        el.innerHTML = `<div class="pdf-page-placeholder" style="height: ${ph}px;"><span class="page-badge">Page ${pageIndex + 1}</span></div>`;
      }
    });
    renderedPages.clear();
  }

  $: if ($pdfZoom !== undefined && doc && !loading) {
    handleZoomChanged();
  }

  function handleZoomChanged() {
    cancelAllRenders();
    computePageTops();
    if (!viewportEl) return;

    for (let i = 0; i < numPages; i++) {
      const el = document.getElementById(`page-${i + 1}`);
      if (!el) continue;
      const scale = getScaleForPage(i);
      const dims = pageDimensions[i];
      if (dims) {
        const pw = Math.floor(dims.width * scale);
        const ph = Math.floor(dims.height * scale);
        el.style.width = `${pw}px`;
        el.style.height = `${ph}px`;
        const placeholder = el.querySelector('.pdf-page-placeholder');
        if (placeholder) placeholder.style.height = `${ph}px`;
      }
    }

    const activeIdx = getActivePageIndex();
    queueVisiblePages(activeIdx);
    scheduleRender();
  }

  function handleResize() {
    if (!viewportEl) return;
    const newWidth = viewportEl.clientWidth;
    if (newWidth > 0 && Math.abs(newWidth - viewportWidth) > 6) {
      viewportWidth = newWidth;
      handleZoomChanged();
    }
  }

  function handleKeydown(e) {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
    if (e.key === 'ArrowRight' || e.key === 'PageDown') {
      e.preventDefault();
      navigate(1);
    } else if (e.key === 'ArrowLeft' || e.key === 'PageUp') {
      e.preventDefault();
      navigate(-1);
    }
  }

  function navigate(delta) {
    const newIndex = $currentSpineIndex + delta;
    if (newIndex < 0 || newIndex >= numPages) return;
    scrollToPage(newIndex + 1);
  }

  function applyHighlightsToPage(pageIndex, textLayerEl) {
    if (!textLayerEl) return;
    const pageHighlights = $highlights.filter(
      (h) => h.bookId === $currentBookId && h.spineIndex === pageIndex
    );
    for (const h of pageHighlights) highlightTextInContainer(textLayerEl, h);
  }

  function getAllTextNodes(node) {
    const nodes = [];
    const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT, null, false);
    let n;
    while ((n = walker.nextNode())) nodes.push(n);
    return nodes;
  }

  function wrapTextNodeRange(node, start, end, id, color, note) {
    try {
      const range = document.createRange();
      range.setStart(node, start);
      range.setEnd(node, end);
      const mark = document.createElement('mark');
      mark.className = `reader-highlight reader-highlight-${color || 'yellow'}`;
      mark.setAttribute('data-highlight-id', id);
      mark.setAttribute('data-color', color || 'yellow');
      if (note) mark.setAttribute('data-note', note);
      mark.addEventListener('click', (e) => { e.stopPropagation(); openNotePopover(mark, id); });
      const frag = range.extractContents();
      mark.appendChild(frag);
      range.insertNode(mark);
    } catch (_) {}
  }

  function highlightTextInContainer(container, h) {
    const targetText = (h.text || '').trim();
    if (!targetText) return;

    const textNodes = getAllTextNodes(container);
    if (!textNodes.length) return;

    for (let i = 0; i < textNodes.length; i++) {
      const node = textNodes[i];
      if (node.parentElement?.classList.contains('reader-highlight')) continue;
      const idx = node.nodeValue.indexOf(targetText);
      if (idx !== -1) {
        wrapTextNodeRange(node, idx, idx + targetText.length, h.id, h.color, h.note);
        return;
      }
    }

    let fullText = '';
    const nodeMap = [];
    for (const tNode of textNodes) {
      if (tNode.parentElement?.classList.contains('reader-highlight')) continue;
      const val = tNode.nodeValue;
      for (let o = 0; o < val.length; o++) { nodeMap.push({ node: tNode, offset: o }); fullText += val[o]; }
    }

    const cleanTarget = targetText.replace(/\s+/g, ' ');
    let normFull = '', inSpace = false;
    const normToOrigMap = [];
    for (let c = 0; c < fullText.length; c++) {
      const ch = fullText[c];
      if (/\s/.test(ch)) {
        if (!inSpace) { normToOrigMap.push(c); normFull += ' '; inSpace = true; }
      } else { normToOrigMap.push(c); normFull += ch; inSpace = false; }
    }

    let foundIdx = normFull.indexOf(cleanTarget);
    if (foundIdx === -1) foundIdx = normFull.toLowerCase().indexOf(cleanTarget.toLowerCase());
    if (foundIdx === -1) return;

    const normEndIdx = foundIdx + cleanTarget.length - 1;
    const startCharIdx = normToOrigMap[foundIdx];
    const endCharIdx = (normToOrigMap[normEndIdx] || startCharIdx) + 1;

    const nodeSpans = [];
    let cur = null, sOffset = 0, eOffset = 0;
    for (let i = startCharIdx; i < endCharIdx && i < nodeMap.length; i++) {
      const item = nodeMap[i];
      if (item.node !== cur) {
        if (cur) nodeSpans.push({ node: cur, start: sOffset, end: eOffset });
        cur = item.node; sOffset = item.offset; eOffset = item.offset + 1;
      } else { eOffset = item.offset + 1; }
    }
    if (cur) nodeSpans.push({ node: cur, start: sOffset, end: eOffset });

    for (let s = nodeSpans.length - 1; s >= 0; s--) {
      const span = nodeSpans[s];
      wrapTextNodeRange(span.node, span.start, span.end, h.id, h.color, h.note);
    }
  }

  function handleSelectionChange() {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !sel.rangeCount) {
      if (selectionToolbar.visible) selectionToolbar.visible = false;
      return;
    }

    const text = sel.toString().trim();
    if (!text || text.length < 1) { selectionToolbar.visible = false; return; }

    const range = sel.getRangeAt(0);
    const container = range.commonAncestorContainer;
    const pageEl = (container.nodeType === 1 ? container : container.parentElement)?.closest('.pdf-page-container');

    if (!pageEl || !viewportEl.contains(pageEl)) { selectionToolbar.visible = false; return; }

    const pageIndex = parseInt(pageEl.getAttribute('data-page-index'), 10);
    const rect = range.getBoundingClientRect();
    const vpRect = viewportEl.getBoundingClientRect();

    selectionToolbar = {
      visible: true,
      x: rect.left - vpRect.left + rect.width / 2,
      y: rect.top - vpRect.top - 8,
      pageIndex,
      text,
      range,
    };
  }

  async function createHighlight(color) {
    if (!selectionToolbar.visible || !selectionToolbar.text) return;

    const newH = {
      id: crypto.randomUUID ? crypto.randomUUID() : 'hl_' + Date.now(),
      bookId: $currentBookId,
      spineIndex: selectionToolbar.pageIndex,
      text: selectionToolbar.text,
      color,
      note: '',
      createdAt: new Date().toISOString(),
    };

    await addHighlight(newH);
    highlights.update((items) => [newH, ...items]);

    const textLayerEl = document.getElementById(`page-${selectionToolbar.pageIndex + 1}`)?.querySelector('.textLayer');
    if (textLayerEl) highlightTextInContainer(textLayerEl, newH);

    window.getSelection()?.removeAllRanges();
    selectionToolbar.visible = false;
  }

  async function copySelectedText() {
    if (selectionToolbar.text) navigator.clipboard.writeText(selectionToolbar.text);
    window.getSelection()?.removeAllRanges();
    selectionToolbar.visible = false;
  }

  function openNotePopover(markEl, highlightId) {
    const h = $highlights.find((item) => item.id === highlightId);
    if (!h) return;
    const rect = markEl.getBoundingClientRect();
    const vpRect = viewportEl.getBoundingClientRect();
    activeNotePopover = {
      highlightId,
      highlight: h,
      note: h.note || '',
      draft: h.note || '',
      x: Math.min(vpRect.width - 280, Math.max(16, rect.left - vpRect.left)),
      y: rect.bottom - vpRect.top + 8,
    };
  }

  async function saveNoteDraft() {
    if (!activeNotePopover) return;
    const { highlightId, draft } = activeNotePopover;
    await updateHighlightNote(highlightId, draft);
    highlights.update((items) => items.map((item) => (item.id === highlightId ? { ...item, note: draft } : item)));
    const marks = viewportEl.querySelectorAll(`[data-highlight-id="${highlightId}"]`);
    marks.forEach((m) => m.setAttribute('data-note', draft));
    activeNotePopover = null;
  }

  async function handleDeleteHighlight(highlightId) {
    await deleteHighlight(highlightId);
    highlights.update((items) => items.filter((item) => item.id !== highlightId));
    const marks = viewportEl.querySelectorAll(`[data-highlight-id="${highlightId}"]`);
    marks.forEach((mark) => {
      const parent = mark.parentNode;
      while (mark.firstChild) parent.insertBefore(mark.firstChild, mark);
      parent.removeChild(mark);
    });
    activeNotePopover = null;
  }

  function scrollToPage(pageNumber, offsetTop = 0) {
    if (!viewportEl) return;
    const pageIndex = Math.max(0, Math.min(numPages - 1, pageNumber - 1));
    const targetScroll = (pageTops[pageIndex] || 0) + offsetTop;
    viewportEl.scrollTo({ top: targetScroll, behavior: 'smooth' });
    currentSpineIndex.set(pageIndex);
    updateMinutesRemaining(pageIndex);
    queueVisiblePages(pageIndex);
    scheduleRender();
  }

  function handleScrollToPageEvent(e) {
    scrollToPage((e.detail?.pageIndex ?? 0) + 1);
  }

  async function handleScrollToTocEvent(e) {
    const { pageIndex, title } = e.detail || {};
    if (pageIndex === undefined) return;

    scrollToPage(pageIndex + 1);

    if (title) {
      const cleanTitle = title.replace(/^[0-9.]+\s*/, '').trim().toLowerCase();
      const fullTitle = title.trim().toLowerCase();

      const findAndScrollToHeading = (attempts = 0) => {
        const textLayer = document.getElementById(`page-${pageIndex + 1}`)?.querySelector('.textLayer');
        if (textLayer) {
          const spans = textLayer.querySelectorAll('span');
          let matchedSpan = null;
          for (const span of spans) {
            const txt = span.textContent.trim().toLowerCase();
            if (txt && (txt.includes(cleanTitle) || cleanTitle.includes(txt) || txt.includes(fullTitle))) {
              matchedSpan = span;
              break;
            }
          }
          if (matchedSpan) {
            matchedSpan.scrollIntoView({ behavior: 'smooth', block: 'center' });
            matchedSpan.classList.add('pdf-search-pulse');
            setTimeout(() => matchedSpan.classList.remove('pdf-search-pulse'), 2000);
            return;
          }
        }
        if (attempts < 8) {
          setTimeout(() => findAndScrollToHeading(attempts + 1), 120);
        }
      };
      setTimeout(() => findAndScrollToHeading(0), 100);
    }
  }

  function handleScrollToBookmarkEvent(e) {
    scrollToPage((e.detail?.pageIndex ?? 0) + 1, e.detail?.scrollOffset || 0);
  }

  async function handleJumpToHighlightEvent(e) {
    const { id, spineIndex } = e.detail || {};
    const pageIndex = spineIndex !== undefined ? spineIndex : 0;
    scrollToPage(pageIndex + 1);

    const checkHighlight = (attempts = 0) => {
      const mark = viewportEl?.querySelector(`[data-highlight-id="${id}"]`);
      if (mark) {
        mark.scrollIntoView({ behavior: 'smooth', block: 'center' });
        mark.classList.add('pulse-highlight');
        setTimeout(() => mark.classList.remove('pulse-highlight'), 2400);
      } else {
        const textLayer = document.getElementById(`page-${pageIndex + 1}`)?.querySelector('.textLayer');
        if (textLayer) {
          highlightTextInContainer(textLayer, e.detail);
          const newMark = viewportEl?.querySelector(`[data-highlight-id="${id}"]`);
          if (newMark) {
            newMark.scrollIntoView({ behavior: 'smooth', block: 'center' });
            newMark.classList.add('pulse-highlight');
            setTimeout(() => newMark.classList.remove('pulse-highlight'), 2400);
            return;
          }
        }
        if (attempts < 10) {
          setTimeout(() => checkHighlight(attempts + 1), 150);
        }
      }
    };
    setTimeout(() => checkHighlight(0), 100);
  }

  function handleRemoveHighlightEvent(e) {
    const { highlightId } = e.detail || {};
    if (!highlightId) return;
    const marks = viewportEl?.querySelectorAll(`[data-highlight-id="${highlightId}"]`);
    marks?.forEach((mark) => {
      const parent = mark.parentNode;
      while (mark.firstChild) parent.insertBefore(mark.firstChild, mark);
      parent.removeChild(mark);
    });
  }

  async function handleFindAndScrollEvent(e) {
    const { pageIndex, query } = e.detail || {};
    if (pageIndex === undefined || !query) return;
    scrollToPage(pageIndex + 1);

    const checkSearch = (attempts = 0) => {
      const textLayer = document.getElementById(`page-${pageIndex + 1}`)?.querySelector('.textLayer');
      if (textLayer) {
        const spans = textLayer.querySelectorAll('span');
        const q = query.toLowerCase();
        for (const span of spans) {
          if (span.textContent.toLowerCase().includes(q)) {
            span.scrollIntoView({ behavior: 'smooth', block: 'center' });
            span.classList.add('pdf-search-pulse');
            setTimeout(() => span.classList.remove('pdf-search-pulse'), 2500);
            return;
          }
        }
      }
      if (attempts < 10) {
        setTimeout(() => checkSearch(attempts + 1), 150);
      }
    };
    setTimeout(() => checkSearch(0), 100);
  }

  function handleZoomIn() {
    pdfZoom.update((z) => (z === -1 ? 125 : Math.min(250, z + 25)));
  }

  function handleZoomOut() {
    pdfZoom.update((z) => (z === -1 ? 80 : Math.max(50, z - 25)));
  }

  function handleZoomFit() {
    pdfZoom.set(-1);
  }
</script>

<div class="pdf-reader-pane" role="region" aria-label="PDF Document Viewer">
  {#if loading}
    <div class="pdf-center-state">
      <div class="pdf-spinner"></div>
      <p class="pdf-loading-text">Loading document...</p>
    </div>
  {:else if error}
    <div class="pdf-center-state">
      <div class="pdf-error-icon">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
      </div>
      <p class="pdf-error-title">Failed to load PDF</p>
      <p class="pdf-error-msg">{error}</p>
      <button class="pdf-retry-btn" on:click={loadPDFDocument}>Try Again</button>
    </div>
  {/if}

  <div
    class="pdf-viewport"
    class:pdf-viewport-hidden={loading || error}
    bind:this={viewportEl}
    on:scroll={handleScroll}
  >
    <div class="pdf-pages-list">
      {#each Array(numPages) as _, i (i)}
        {@const dims = pageDimensions[i] || { width: 612, height: 792 }}
        {@const scale = getScaleForPage(i)}
        {@const pw = Math.floor(dims.width * scale)}
        {@const ph = Math.floor(dims.height * scale)}
        <div
          class="pdf-page-container"
          data-page-index={i}
          id="page-{i + 1}"
          style="width: {pw}px; height: {ph}px;"
        >
          <div class="pdf-page-placeholder" style="height: {ph}px;">
            <span class="page-badge">Page {i + 1}</span>
          </div>
        </div>
      {/each}
    </div>

    {#if selectionToolbar.visible}
      <div
        class="reader-selection-menu"
        style="top: {selectionToolbar.y}px; left: {selectionToolbar.x}px; transform: translate(-50%, -100%);"
      >
        {#each colors as c}
          <button
            class="reader-color-btn"
            style="background-color: {c.hex};"
            title="Highlight {c.label}"
            on:mousedown|preventDefault
            on:click={() => createHighlight(c.id)}
          ></button>
        {/each}

        <div class="reader-menu-divider"></div>

        <button
          class="reader-icon-btn"
          title="Copy Text"
          on:mousedown|preventDefault
          on:click={copySelectedText}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
          </svg>
        </button>
      </div>
    {/if}

    {#if activeNotePopover}
      <div
        class="reader-inline-note-box"
        style="top: {activeNotePopover.y}px; left: {activeNotePopover.x}px;"
      >
        <div class="note-box-header">
          <span class="note-box-title">Note</span>
          <button
            class="reader-delete-btn"
            title="Delete Highlight"
            on:click={() => handleDeleteHighlight(activeNotePopover.highlightId)}
          >
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="3 6 5 6 21 6"></polyline>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
            </svg>
          </button>
        </div>
        <textarea
          bind:value={activeNotePopover.draft}
          placeholder="Add note or thoughts..."
          rows="3"
        ></textarea>
        <div class="note-btn-row">
          <button class="btn-cancel" on:click={() => activeNotePopover = null}>Cancel</button>
          <button class="btn-save-note" on:click={saveNoteDraft}>Save</button>
        </div>
      </div>
    {/if}

    <div class="pdf-nav">
      <button
        class="nav-btn prev"
        class:disabled={$currentSpineIndex <= 0}
        on:click={() => navigate(-1)}
        title="Previous page"
        id="prev-pdf-page-btn"
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m15 18-6-6 6-6"/>
        </svg>
      </button>
      <button
        class="nav-btn next"
        class:disabled={$currentSpineIndex >= numPages - 1}
        on:click={() => navigate(1)}
        title="Next page"
        id="next-pdf-page-btn"
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m9 18 6-6-6-6"/>
        </svg>
      </button>
    </div>
  </div>
</div>

<style>
  .pdf-reader-pane {
    position: relative;
    flex: 1;
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    overflow: hidden;
  }

  .pdf-center-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 32px;
    text-align: center;
  }

  .pdf-spinner {
    width: 32px;
    height: 32px;
    border: 3px solid var(--border-subtle);
    border-top-color: var(--accent-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .pdf-loading-text {
    font-size: 0.875rem;
    color: var(--fg-secondary);
    font-weight: 500;
  }

  .pdf-error-icon { color: #ef4444; }

  .pdf-error-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--fg-primary);
  }

  .pdf-error-msg {
    font-size: 0.875rem;
    color: var(--fg-secondary);
    max-width: 400px;
  }

  .pdf-retry-btn {
    padding: 6px 16px;
    border-radius: 8px;
    background: var(--accent-primary);
    color: white;
    font-size: 0.875rem;
    font-weight: 500;
    border: none;
    cursor: pointer;
    margin-top: 8px;
  }

  .pdf-viewport {
    flex: 1;
    overflow-y: scroll;
    overflow-x: auto;
    position: relative;
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
  }

  .pdf-viewport-hidden {
    display: none !important;
  }

  .pdf-pages-list {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 24px;
    padding: 24px 0 80px;
  }

  .pdf-page-container {
    position: relative;
    background: #ffffff;
    border: 1px solid var(--border-subtle);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    border-radius: 4px;
    margin: 0 auto;
    box-sizing: content-box;
    flex-shrink: 0;
  }

  .pdf-page-placeholder {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #f8fafc;
    color: #94a3b8;
  }

  .page-badge {
    font-size: 0.8125rem;
    font-weight: 600;
    color: #94a3b8;
    background: rgba(0, 0, 0, 0.04);
    padding: 4px 12px;
    border-radius: 12px;
  }

  :global(.pdf-canvas) {
    display: block;
  }

  :global(.textLayer) {
    position: absolute;
    top: 0;
    left: 0;
    overflow: hidden;
    opacity: 1;
  }

  :global(.textLayer ::selection) {
    background: rgba(99, 102, 241, 0.35);
  }

  :global(mark.reader-highlight) {
    border-radius: 2px;
    cursor: pointer;
    transition: filter 0.15s ease;
    display: inline;
    padding: 1px 0;
  }

  :global(mark.reader-highlight:hover) {
    filter: brightness(0.9);
  }

  :global(mark.reader-highlight-yellow) {
    background-color: rgba(250, 204, 21, 0.6) !important;
    color: inherit;
  }
  :global(mark.reader-highlight-green) {
    background-color: rgba(74, 222, 128, 0.55) !important;
    color: inherit;
  }
  :global(mark.reader-highlight-blue) {
    background-color: rgba(96, 165, 250, 0.55) !important;
    color: inherit;
  }
  :global(mark.reader-highlight-purple) {
    background-color: rgba(192, 132, 252, 0.55) !important;
    color: inherit;
  }
  :global(mark.reader-highlight-pink) {
    background-color: rgba(251, 113, 133, 0.55) !important;
    color: inherit;
  }

  :global(.pulse-highlight) {
    animation: highlightPulse 2.4s ease-out forwards;
  }

  @keyframes highlightPulse {
    0% { outline: 3px solid #6366f1; transform: scale(1.02); }
    40% { outline: 3px solid rgba(99, 102, 241, 0.6); }
    100% { outline: 0px solid transparent; transform: scale(1); }
  }

  :global(.pdf-search-pulse) {
    animation: searchPulse 2.5s ease-out forwards;
    background-color: rgba(245, 158, 11, 0.6) !important;
    border-radius: 2px;
  }

  @keyframes searchPulse {
    0% { outline: 3px solid #f59e0b; }
    100% { outline: 0px solid transparent; }
  }

  .reader-selection-menu {
    position: absolute;
    z-index: 10000;
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 6px 10px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 28px;
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.28), 0 2px 8px rgba(0, 0, 0, 0.08);
    user-select: none;
    pointer-events: auto;
    animation: menuPopIn 0.16s cubic-bezier(0.16, 1, 0.3, 1);
  }

  @keyframes menuPopIn {
    0% { transform: translate(-50%, -100%) scale(0.85); opacity: 0; }
    100% { transform: translate(-50%, -100%) scale(1); opacity: 1; }
  }

  .reader-color-btn {
    width: 22px;
    height: 22px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    padding: 0;
    transition: transform 0.12s ease, border-color 0.12s ease;
  }

  .reader-color-btn:hover {
    transform: scale(1.25);
    border-color: var(--fg-primary);
  }

  .reader-menu-divider {
    width: 1px;
    height: 16px;
    background: var(--border-subtle);
    margin: 0 2px;
  }

  .reader-icon-btn {
    background: transparent;
    border: none;
    color: var(--fg-secondary);
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
    color: var(--accent-primary);
  }

  .reader-inline-note-box {
    position: absolute;
    z-index: 10001;
    width: 270px;
    padding: 12px;
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    border-radius: 12px;
    box-shadow: 0 12px 28px rgba(0, 0, 0, 0.28);
    display: flex;
    flex-direction: column;
    gap: 8px;
    animation: noteFadeIn 0.15s ease-out;
  }

  @keyframes noteFadeIn {
    0% { transform: scale(0.95); opacity: 0; }
    100% { transform: scale(1); opacity: 1; }
  }

  .note-box-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .note-box-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--fg-primary);
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
    opacity: 0.8;
  }

  .reader-delete-btn:hover {
    opacity: 1;
    transform: scale(1.1);
  }

  .reader-inline-note-box textarea {
    width: 100%;
    border: 1px solid var(--border-subtle);
    background: var(--bg-primary);
    color: var(--fg-primary);
    font-family: inherit;
    font-size: 0.8125rem;
    padding: 8px;
    border-radius: 6px;
    resize: none;
    outline: none;
    box-sizing: border-box;
  }

  .reader-inline-note-box textarea:focus {
    border-color: var(--accent-primary);
  }

  .note-btn-row {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }

  .note-btn-row button {
    font-size: 0.75rem;
    padding: 4px 10px;
    border-radius: 6px;
    border: none;
    cursor: pointer;
    font-weight: 500;
  }

  .btn-cancel {
    background: transparent;
    color: var(--fg-secondary);
  }

  .btn-cancel:hover {
    color: var(--fg-primary);
  }

  .btn-save-note {
    background: var(--accent-primary);
    color: white;
  }

  .pdf-nav {
    position: fixed;
    bottom: 24px;
    right: 24px;
    display: flex;
    gap: 8px;
    z-index: 50;
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
