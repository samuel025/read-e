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
    chapterPageInfo,
    highlights,
    settings,
    pdfDoc,
    pdfZoom,
    library,
  } from '../stores/app.js';
  import {
    getPDFData,
    saveProgress,
    getProgress,
    addHighlight,
    deleteHighlight,
    updateHighlightNote,
    updateBookTotalCount,
  } from './api.js';
  import { getDocumentPageTexts } from './pdfSearch.js';
  import DictionaryPopover from './DictionaryPopover.svelte';

  pdfjsLib.GlobalWorkerOptions.workerSrc = pdfjsWorker;

  let paneEl;
  let viewportEl;
  let viewportWidth = 0;
  let loading = true;
  let error = null;
  let doc = null;
  let currentLoadingTask = null;
  let numPages = 0;
  let pageDimensions = [];
  let pageTops = [];

  let renderedPages = new Map();
  let renderQueue = [];
  let renderRAFId = null;
  let activeRenders = 0;
  const MAX_CONCURRENT_RENDERS = 1;
  const PRE_RENDER_AHEAD = 1;
  const PRE_RENDER_BEHIND = 0;
  const EVICT_BEHIND = 2;
  const EVICT_AHEAD = 3;
  let scrollRAF = null;
  let saveProgressTimer = null;
  let resizeObserver = null;
  let isRestoringPosition = false;
  let activeDictionary = null;

  let selectionToolbar = {
    visible: false,
    x: 0,
    y: 0,
    transform: 'translate(-50%, -100%)',
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

    window.addEventListener('mouseup', handleMouseUp);
    window.addEventListener('mousedown', handleMouseDown);
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
    window.removeEventListener('mouseup', handleMouseUp);
    window.removeEventListener('mousedown', handleMouseDown);

    try {
      if (scrollRAF) cancelAnimationFrame(scrollRAF);
      if (saveProgressTimer) {
        clearTimeout(saveProgressTimer);
        saveProgressTimer = null;
      }

      if ($currentBookId && viewportEl && pageTops.length > 0) {
        const activeIndex = getActivePageIndex();
        const pageTop = pageTops[activeIndex] || 0;
        const pageOffset = Math.max(0, Math.round(viewportEl.scrollTop - pageTop));
        saveProgress($currentBookId, activeIndex, pageOffset);
      }

      if (resizeObserver) {
        resizeObserver.disconnect();
        resizeObserver = null;
      }

      cancelAllRenders();

      if (currentLoadingTask) {
        try { currentLoadingTask.destroy(); } catch (_) {}
        currentLoadingTask = null;
      }

      if (doc) {
        try { doc.destroy(); } catch (_) {}
        doc = null;
      }

      pdfDoc.set(null);
      pageDimensions = [];
      pageTops = [];
    } catch (e) {
      console.error('Error during PDFPane onDestroy cleanup:', e);
    }
  });

  async function loadPDFDocument() {
    loading = true;
    error = null;
    const bookId = $currentBookId;
    if (!bookId) return;

    try {
      try {
        currentLoadingTask = pdfjsLib.getDocument({
          url: `/pdf/${encodeURIComponent(bookId)}`,
          cMapUrl: 'https://cdn.jsdelivr.net/npm/pdfjs-dist@3.11.174/cmaps/',
          cMapPacked: true,
        });
        doc = await currentLoadingTask.promise;
      } catch (streamErr) {
        console.warn('Streaming PDF failed, falling back to IPC buffer:', streamErr);
        let data = await getPDFData(bookId);
        if (!data) throw new Error('No PDF data received from backend');

        currentLoadingTask = pdfjsLib.getDocument({
          data,
          cMapUrl: 'https://cdn.jsdelivr.net/npm/pdfjs-dist@3.11.174/cmaps/',
          cMapPacked: true,
        });
        data = null;
        doc = await currentLoadingTask.promise;
      }
      currentLoadingTask = null;
      pdfDoc.set(doc);
      numPages = doc.numPages;
      spineCount.set(numPages);
      if ($currentBookId && numPages > 0) {
        updateBookTotalCount($currentBookId, numPages);
        library.update(lib => lib.map(b => {
          if (b.id === $currentBookId) {
            const progress = b.finished ? 100 : (numPages > 1 && (b.spineIndex || 0) >= numPages - 1 ? 100 : Math.round(((b.spineIndex || 0) + 1) / numPages * 100));
            return { ...b, totalCount: numPages, progress };
          }
          return b;
        }));
      }

      // Sample page 1 for initial aspect ratio without querying all pages upfront
      const samplePage = await doc.getPage(1);
      const sampleVp = samplePage.getViewport({ scale: 1.0 });
      const defaultDim = {
        width: sampleVp.width || 612,
        height: sampleVp.height || 792,
        aspectRatio: (sampleVp.height || 792) / (sampleVp.width || 612),
      };
      samplePage.cleanup();

      const dims = new Array(numPages);
      for (let i = 0; i < numPages; i++) {
        dims[i] = defaultDim;
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

  let avgWordsPerPage = 250;

  async function calculateReadingStats(pdf) {
    try {
      const sampleCount = Math.min(pdf.numPages, 10);
      let sampleWords = 0;
      let countedPages = 0;
      for (let i = 1; i <= sampleCount; i++) {
        try {
          const page = await pdf.getPage(i);
          const textContent = await page.getTextContent();
          const str = textContent.items.map(it => it.str).join(' ');
          const w = str.trim().split(/\s+/).filter(Boolean).length;
          if (w > 20) {
            sampleWords += w;
            countedPages++;
          }
          page.cleanup();
        } catch (_) {}
      }
      if (countedPages > 0) {
        avgWordsPerPage = Math.max(100, Math.round(sampleWords / countedPages));
      } else {
        avgWordsPerPage = 250;
      }
      updateMinutesRemaining($currentSpineIndex);
    } catch (_) {
      avgWordsPerPage = 250;
      updateMinutesRemaining($currentSpineIndex);
    }
  }

  function getTOCChapters() {
    const list = [];
    function traverse(items) {
      if (!items) return;
      for (const item of items) {
        if (typeof item.spineIndex === 'number') {
          list.push({ title: item.title, pageIndex: item.spineIndex });
        }
        if (item.children?.length) {
          traverse(item.children);
        }
      }
    }
    traverse($toc);
    list.sort((a, b) => a.pageIndex - b.pageIndex);
    return list;
  }

  function updateChapterPageInfo(pageIdx) {
    const totalDocPages = numPages || $spineCount || 1;
    const chapters = getTOCChapters();

    let currentChTitle = '';
    let chStartPage = 0;
    let chEndPage = totalDocPages;

    if (chapters.length > 0) {
      for (let i = 0; i < chapters.length; i++) {
        if (chapters[i].pageIndex <= pageIdx) {
          currentChTitle = chapters[i].title;
          chStartPage = chapters[i].pageIndex;
          if (i + 1 < chapters.length) {
            chEndPage = chapters[i + 1].pageIndex;
          } else {
            chEndPage = totalDocPages;
          }
        } else {
          break;
        }
      }
      if (!currentChTitle && chapters[0].pageIndex > pageIdx) {
        currentChTitle = 'Front Matter';
        chStartPage = 0;
        chEndPage = chapters[0].pageIndex;
      }
    }

    if (!currentChTitle) {
      currentChTitle = `Page ${pageIdx + 1}`;
      chStartPage = 0;
      chEndPage = totalDocPages;
    }

    const chapterTotalPages = Math.max(1, chEndPage - chStartPage);
    const currentPageInChapter = Math.min(chapterTotalPages, Math.max(1, pageIdx - chStartPage + 1));
    const pagesLeft = Math.max(0, chEndPage - pageIdx - 1);
    const percentInChapter = Math.min(100, Math.max(0, Math.round((currentPageInChapter / chapterTotalPages) * 100)));

    chapterPageInfo.set({
      currentPage: currentPageInChapter,
      totalPages: chapterTotalPages,
      pagesLeft: pagesLeft,
      chapterTitle: currentChTitle,
      percentInChapter: percentInChapter,
    });
  }

  function updateMinutesRemaining(activePageIndex) {
    const totalPages = numPages || $spineCount || 1;
    const pageIdx = typeof activePageIndex === 'number' ? activePageIndex : ($currentSpineIndex || 0);
    const pagesLeft = Math.max(0, totalPages - 1 - pageIdx);
    const wordsLeft = pagesLeft * avgWordsPerPage;
    const totalWords = totalPages * avgWordsPerPage;
    const bookMinutesLeft = Math.round(wordsLeft / 220);
    const chapterMinutes = Math.max(1, Math.round(avgWordsPerPage / 220));

    readingStats.set({
      words: totalWords,
      remainingWords: wordsLeft,
      minutesLeft: bookMinutesLeft,
      chapterMinutes: chapterMinutes,
    });

    updateChapterPageInfo(pageIdx);
  }

  async function restoreSavedPosition(bookId) {
    try {
      const pos = await getProgress(bookId);
      if (pos && typeof pos.spineIndex === 'number' && pos.spineIndex >= 0) {
        currentSpineIndex.set(pos.spineIndex);
        await tick();
        isRestoringPosition = true;
        scrollToPage(pos.spineIndex + 1, pos.scrollOffset || 0, 'auto');
        setTimeout(() => {
          isRestoringPosition = false;
        }, 400);
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
    if (selectionToolbar.visible) selectionToolbar.visible = false;
    if (activeNotePopover) activeNotePopover = null;

    if (scrollRAF) return;
    scrollRAF = requestAnimationFrame(() => {
      scrollRAF = null;
      if (!viewportEl || !pageTops.length) return;

      const activeIndex = getActivePageIndex();
      if ($currentSpineIndex !== activeIndex) {
        currentSpineIndex.set(activeIndex);
        updateMinutesRemaining(activeIndex);
      }

      queueVisiblePages(activeIndex);
      scheduleRender();

      if (isRestoringPosition) return;

      clearTimeout(saveProgressTimer);
      saveProgressTimer = setTimeout(() => {
        if ($currentBookId && viewportEl && pageTops.length > 0) {
          const pageTop = pageTops[activeIndex] || 0;
          const pageOffset = Math.max(0, Math.round(viewportEl.scrollTop - pageTop));
          saveProgress($currentBookId, activeIndex, pageOffset);

          const isAtBottom = viewportEl.scrollTop + viewportEl.clientHeight >= viewportEl.scrollHeight - 60;
          if ((activeIndex >= numPages - 1 || isAtBottom) && numPages > 1) {
            library.update(lib => lib.map(b => b.id === $currentBookId ? { ...b, finished: true, progress: 100 } : b));
          }
        }
      }, 500);
    });
  }

  function handleMouseDown(e) {
    if (e.target.closest('.reader-selection-menu') || e.target.closest('.reader-inline-note-box')) {
      return;
    }
    if (selectionToolbar.visible) {
      selectionToolbar.visible = false;
    }
    if (activeNotePopover && !e.target.closest('mark.reader-highlight')) {
      activeNotePopover = null;
    }
  }

  function handleMouseUp(e) {
    if (e.target.closest('.reader-selection-menu') || e.target.closest('.reader-inline-note-box')) {
      return;
    }
    setTimeout(handleSelectionChange, 25);
  }

  function queueVisiblePages(activeIdx) {
    // 1. Evict pages far outside the viewport to prevent memory growth
    for (const [pIdx] of renderedPages) {
      if (pIdx < activeIdx - EVICT_BEHIND || pIdx > activeIdx + EVICT_AHEAD) {
        unrenderPage(pIdx);
      }
    }

    // 2. Determine needed window: from (activeIdx - 2) to (activeIdx + 4)
    const needed = [];
    needed.push(activeIdx);
    if (activeIdx + 1 < numPages) needed.push(activeIdx + 1);

    // Pre-render ahead (user usually scrolls forward)
    for (let i = activeIdx + 2; i <= activeIdx + PRE_RENDER_AHEAD && i < numPages; i++) {
      needed.push(i);
    }

    // Pre-render behind
    for (let i = activeIdx - 1; i >= activeIdx - PRE_RENDER_BEHIND && i >= 0; i--) {
      needed.push(i);
    }

    // 3. Queue pages that are not yet rendered or in queue
    for (const idx of needed) {
      if (!renderedPages.has(idx) && !renderQueue.includes(idx)) {
        renderQueue.push(idx);
      }
    }

    renderQueue = renderQueue.filter(idx => (idx >= activeIdx - EVICT_BEHIND && idx <= activeIdx + EVICT_AHEAD) && !renderedPages.has(idx));
  }

  function scheduleRender() {
    if (renderRAFId !== null) return;
    renderRAFId = requestAnimationFrame(processRenderQueue);
  }

  async function processRenderQueue() {
    renderRAFId = null;
    if (!doc || !viewportEl || !renderQueue.length) return;

    const activeIdx = getActivePageIndex();
    // Prioritize active page first, then next page, then nearest
    renderQueue.sort((a, b) => {
      if (a === activeIdx) return -1;
      if (b === activeIdx) return 1;
      if (a === activeIdx + 1) return -1;
      if (b === activeIdx + 1) return 1;
      return Math.abs(a - activeIdx) - Math.abs(b - activeIdx);
    });

    while (activeRenders < MAX_CONCURRENT_RENDERS && renderQueue.length > 0) {
      const pageIndex = renderQueue.shift();
      if (renderedPages.has(pageIndex)) continue;

      activeRenders++;
      renderPage(pageIndex).finally(() => {
        activeRenders--;
        scheduleRender();
      });
    }
  }

  async function renderPage(pageIndex) {
    if (!doc || renderedPages.has(pageIndex)) return;

    const pageContainer = document.getElementById(`page-${pageIndex + 1}`);
    if (!pageContainer) return;

    const pageNumber = pageIndex + 1;
    renderedPages.set(pageIndex, { loading: true });

    let page = null;
    try {
      page = await doc.getPage(pageNumber);
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
        if (canvas) {
          canvas.width = 0;
          canvas.height = 0;
        }
        page.cleanup();
        return;
      }

      // Immediately display canvas, removing placeholder
      pageContainer.innerHTML = '';
      pageContainer.appendChild(canvas);

      // Mount text layer and highlights in background without blocking next page canvas render
      (async () => {
        try {
          const textLayerDiv = document.createElement('div');
          textLayerDiv.className = 'textLayer';
          textLayerDiv.style.width = `${pw}px`;
          textLayerDiv.style.height = `${ph}px`;
          textLayerDiv.style.setProperty('--scale-factor', viewport.scale);
          pageContainer.appendChild(textLayerDiv);

          const textContent = await page.getTextContent();
          if (renderedPages.has(pageIndex)) {
            const textLayerTask = pdfjsLib.renderTextLayer({
              textContentSource: textContent,
              container: textLayerDiv,
              viewport,
            });
            await textLayerTask.promise;
            applyHighlightsToPage(pageIndex, textLayerDiv);
          }
        } catch (_) {
        } finally {
          page.cleanup();
        }
      })();
    } catch (err) {
      if (err.name !== 'RenderingCancelledException') {
        console.error(`Error rendering page ${pageNumber}:`, err);
      }
      renderedPages.delete(pageIndex);
      if (page) page.cleanup();
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
      if (pageData.canvas) {
        pageData.canvas.width = 0;
        pageData.canvas.height = 0;
      }
      if (pageData.page) {
        try {
          pageData.page.cleanup();
        } catch (_) {}
      }
      renderedPages.delete(pageIndex);
    }

    const pageContainer = document.getElementById(`page-${pageIndex + 1}`);
    if (pageContainer) {
      const cvs = pageContainer.querySelector('canvas');
      if (cvs) {
        cvs.width = 0;
        cvs.height = 0;
      }
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
    activeRenders = 0;
    if (renderRAFId !== null) {
      cancelAnimationFrame(renderRAFId);
      renderRAFId = null;
    }
    renderedPages.forEach((data, pageIndex) => {
      if (data.renderTask) {
        try {
          data.renderTask.cancel();
        } catch (_) {}
      }
      if (data.canvas) {
        data.canvas.width = 0;
        data.canvas.height = 0;
      }
      if (data.page) {
        try {
          data.page.cleanup();
        } catch (_) {}
      }
      const el = document.getElementById(`page-${pageIndex + 1}`);
      if (el) {
        const cvs = el.querySelector('canvas');
        if (cvs) {
          cvs.width = 0;
          cvs.height = 0;
        }
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

  $: if ($highlights && renderedPages.size > 0) {
    for (const [pageIndex, pageData] of renderedPages) {
      if (pageData.textLayer) {
        applyHighlightsToPage(pageIndex, pageData.textLayer);
      }
    }
  }

  function handleZoomChanged() {
    const activeIdx = $currentSpineIndex; // Preserve current page before zoom changes layout
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

    // Restore scroll position to the same logical page
    if (pageTops[activeIdx] !== undefined) {
      viewportEl.scrollTop = pageTops[activeIdx];
    }

    queueVisiblePages(activeIdx);
    scheduleRender();
  }

  function handleResize() {
    if (!viewportEl) return;
    const newWidth = viewportEl.clientWidth;
    if (newWidth > 0 && Math.abs(newWidth - viewportWidth) > 6) {
      viewportWidth = newWidth;

      const isMaximized = window.outerWidth >= window.screen.availWidth * 0.98 || window.innerWidth >= 1400;
      const isDefault = window.innerWidth <= 1250;

      let zoomUpdated = false;
      if (isMaximized && $pdfZoom !== 50) {
        pdfZoom.set(50);
        zoomUpdated = true;
      } else if (isDefault && $pdfZoom !== 100) {
        pdfZoom.set(100);
        zoomUpdated = true;
      }

      if (!zoomUpdated) {
        handleZoomChanged();
      }
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

  function wrapRangeWithMark(range, id, color, note) {
    try {
      if (range.startContainer === range.endContainer && range.startContainer.nodeType === Node.TEXT_NODE) {
        wrapTextNodeRange(range.startContainer, range.startOffset, range.endOffset, id, color, note);
        return true;
      }

      const commonAncestor = range.commonAncestorContainer;
      const root = commonAncestor.nodeType === Node.TEXT_NODE ? commonAncestor.parentElement : commonAncestor;
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, null, false);
      const textNodes = [];
      let curr;
      while ((curr = walker.nextNode())) {
        if (range.intersectsNode(curr)) {
          textNodes.push(curr);
        }
      }

      if (!textNodes.length) return false;

      for (let i = textNodes.length - 1; i >= 0; i--) {
        const node = textNodes[i];
        const start = (node === range.startContainer) ? range.startOffset : 0;
        const end = (node === range.endContainer) ? range.endOffset : node.nodeValue.length;
        if (start < end) {
          wrapTextNodeRange(node, start, end, id, color, note);
        }
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  function highlightTextInContainer(container, h) {
    if (!container || !h || !h.id) return;
    if (container.querySelector(`[data-highlight-id="${h.id}"]`)) return;

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

    const charMap = [];
    let strippedDom = '';
    for (const tNode of textNodes) {
      if (tNode.parentElement?.classList.contains('reader-highlight')) continue;
      const val = tNode.nodeValue;
      for (let o = 0; o < val.length; o++) {
        const ch = val[o];
        if (!/\s/.test(ch)) {
          charMap.push({ node: tNode, offset: o });
          strippedDom += ch;
        }
      }
    }

    const strippedTarget = targetText.replace(/\s+/g, '');
    if (!strippedTarget) return;

    let foundIdx = strippedDom.indexOf(strippedTarget);
    if (foundIdx === -1) foundIdx = strippedDom.toLowerCase().indexOf(strippedTarget.toLowerCase());
    if (foundIdx === -1) return;

    const endCharIdx = foundIdx + strippedTarget.length;
    const nodeSpans = [];
    let cur = null, sOffset = 0, eOffset = 0;
    for (let i = foundIdx; i < endCharIdx && i < charMap.length; i++) {
      const item = charMap[i];
      if (item.node !== cur) {
        if (cur) nodeSpans.push({ node: cur, start: sOffset, end: eOffset });
        cur = item.node;
        sOffset = item.offset;
        eOffset = item.offset + 1;
      } else {
        eOffset = item.offset + 1;
      }
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
    if (!text || text.length < 1) {
      if (selectionToolbar.visible) selectionToolbar.visible = false;
      return;
    }

    const range = sel.getRangeAt(0);
    const startEl = range.startContainer.nodeType === 1 ? range.startContainer : range.startContainer.parentElement;
    const pageEl = startEl?.closest('.pdf-page-container');

    if (!pageEl || !viewportEl || !viewportEl.contains(pageEl)) {
      if (selectionToolbar.visible) selectionToolbar.visible = false;
      return;
    }

    const pageIndex = parseInt(pageEl.getAttribute('data-page-index'), 10);
    let rect = range.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) {
      const clientRects = range.getClientRects();
      if (clientRects.length > 0) {
        rect = clientRects[0];
      } else if (startEl) {
        rect = startEl.getBoundingClientRect();
      }
    }

    const paneRect = paneEl ? paneEl.getBoundingClientRect() : viewportEl.getBoundingClientRect();
    const isNearTop = rect.top - paneRect.top < 50;

    selectionToolbar = {
      visible: true,
      x: Math.max(100, Math.min(paneRect.width - 100, rect.left - paneRect.left + rect.width / 2)),
      y: isNearTop ? rect.bottom - paneRect.top + 8 : rect.top - paneRect.top - 8,
      transform: isNearTop ? 'translate(-50%, 0)' : 'translate(-50%, -100%)',
      pageIndex,
      text,
      range: range.cloneRange(),
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

    let applied = false;
    if (selectionToolbar.range) {
      applied = wrapRangeWithMark(selectionToolbar.range, newH.id, newH.color, newH.note);
    }
    if (!applied) {
      const textLayerEl = document.getElementById(`page-${selectionToolbar.pageIndex + 1}`)?.querySelector('.textLayer');
      if (textLayerEl) highlightTextInContainer(textLayerEl, newH);
    }

    window.getSelection()?.removeAllRanges();
    selectionToolbar.visible = false;
  }

  async function copySelectedText() {
    if (selectionToolbar.text) navigator.clipboard.writeText(selectionToolbar.text);
    window.getSelection()?.removeAllRanges();
    selectionToolbar.visible = false;
  }

  function openDictionaryFromSelection() {
    if (!selectionToolbar.visible || !selectionToolbar.text) return;
    const text = selectionToolbar.text.trim();
    const firstWord = text.split(/\s+/)[0];
    activeDictionary = {
      word: firstWord,
      x: selectionToolbar.x,
      y: selectionToolbar.y,
      placement: 'top'
    };
    window.getSelection()?.removeAllRanges();
    selectionToolbar.visible = false;
  }

  function changeHighlightColor(highlightId, newColor) {
    highlights.update((items) =>
      items.map((item) => (item.id === highlightId ? { ...item, color: newColor } : item))
    );
    if (activeNotePopover) {
      activeNotePopover.highlight = { ...activeNotePopover.highlight, color: newColor };
    }
    const marks = viewportEl?.querySelectorAll(`[data-highlight-id="${highlightId}"]`);
    marks?.forEach((mark) => {
      mark.className = `reader-highlight reader-highlight-${newColor}`;
      mark.setAttribute('data-color', newColor);
    });
    const h = $highlights.find((item) => item.id === highlightId);
    if (h) addHighlight({ ...h, color: newColor });
  }

  function openNotePopover(markEl, highlightId) {
    const h = $highlights.find((item) => item.id === highlightId);
    if (!h) return;
    const rect = markEl.getBoundingClientRect();
    const paneRect = paneEl ? paneEl.getBoundingClientRect() : viewportEl.getBoundingClientRect();
    activeNotePopover = {
      highlightId,
      highlight: h,
      note: h.note || '',
      draft: h.note || '',
      x: Math.min(paneRect.width - 280, Math.max(16, rect.left - paneRect.left)),
      y: Math.min(paneRect.height - 180, rect.bottom - paneRect.top + 8),
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

  function scrollToPage(pageNumber, offsetTop = 0, behavior = 'smooth') {
    if (!viewportEl) return;
    const pageIndex = Math.max(0, Math.min(numPages - 1, pageNumber - 1));
    const pageTop = pageTops[pageIndex] || 0;
    let targetScroll;
    if (offsetTop >= pageTop && pageTop > 0) {
      targetScroll = offsetTop;
    } else {
      targetScroll = pageTop + Math.max(0, offsetTop);
    }
    viewportEl.scrollTo({ top: targetScroll, behavior });
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
    if (pageIndex === undefined || pageIndex === null || isNaN(pageIndex)) return;

    const targetPage = Math.max(1, Math.min(pageIndex + 1, numPages || (pageIndex + 1)));
    scrollToPage(targetPage);

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

<div class="pdf-reader-pane" bind:this={paneEl} role="region" aria-label="PDF Document Viewer">
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

  {#if selectionToolbar.visible}
    <div
      class="reader-selection-menu"
      style="top: {selectionToolbar.y}px; left: {selectionToolbar.x}px; transform: {selectionToolbar.transform || 'translate(-50%, -100%)'};"
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
        title="Define Word"
        on:mousedown|preventDefault
        on:click={openDictionaryFromSelection}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path>
          <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path>
        </svg>
      </button>

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
        <div class="note-color-row">
          {#each colors as c}
            <button
              class="reader-color-btn"
              class:selected={activeNotePopover.highlight?.color === c.id}
              style="background-color: {c.hex};"
              title="Change to {c.label}"
              on:click={() => changeHighlightColor(activeNotePopover.highlightId, c.id)}
            ></button>
          {/each}
        </div>
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
    background: var(--bg-surface, #ffffff);
    color: var(--fg-tertiary, #94a3b8);
    border-radius: 4px;
  }

  .page-badge {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--fg-tertiary, #94a3b8);
    background: rgba(128, 128, 128, 0.08);
    padding: 6px 14px;
    border-radius: 12px;
    opacity: 0.6;
  }

  :global(.pdf-canvas) {
    display: block;
    user-select: none;
    -webkit-user-select: none;
    pointer-events: none;
  }

  :global(.textLayer) {
    position: absolute;
    top: 0;
    left: 0;
    overflow: hidden;
    opacity: 1 !important;
    user-select: text;
    -webkit-user-select: text;
    pointer-events: auto;
    z-index: 2;
  }

  :global(.textLayer),
  :global(.textLayer *),
  :global(.textLayer span),
  :global(.textLayer mark),
  :global(.textLayer br) {
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
    text-shadow: none !important;
  }

  :global(.textLayer span),
  :global(.textLayer mark) {
    user-select: text;
    -webkit-user-select: text;
    pointer-events: auto;
  }

  :global(.textLayer ::selection),
  :global(.textLayer *::selection) {
    background: rgba(99, 102, 241, 0.35) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
    text-shadow: none !important;
  }

  :global(.textLayer ::-moz-selection),
  :global(.textLayer *::-moz-selection) {
    background: rgba(99, 102, 241, 0.35) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
    text-shadow: none !important;
  }

  :global(mark.reader-highlight) {
    border-radius: 2px;
    cursor: pointer;
    transition: filter 0.15s ease;
    display: inline;
    padding: 1px 0;
    pointer-events: auto;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
  }

  :global(mark.reader-highlight:hover) {
    filter: brightness(0.9);
  }

  :global(mark.reader-highlight-yellow) {
    background-color: rgba(250, 204, 21, 0.6) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
  }
  :global(mark.reader-highlight-green) {
    background-color: rgba(74, 222, 128, 0.55) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
  }
  :global(mark.reader-highlight-blue) {
    background-color: rgba(96, 165, 250, 0.55) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
  }
  :global(mark.reader-highlight-purple) {
    background-color: rgba(192, 132, 252, 0.55) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
  }
  :global(mark.reader-highlight-pink) {
    background-color: rgba(251, 113, 133, 0.55) !important;
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
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
    color: transparent !important;
    -webkit-text-fill-color: transparent !important;
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
    0% { opacity: 0; }
    100% { opacity: 1; }
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

  .reader-color-btn:hover,
  .reader-color-btn.selected {
    transform: scale(1.2);
    border-color: var(--fg-primary);
  }

  .note-color-row {
    display: flex;
    align-items: center;
    gap: 6px;
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
