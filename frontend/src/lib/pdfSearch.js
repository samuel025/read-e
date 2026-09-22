// PDF full-text search helper

const pageTextCache = new WeakMap(); // pdfDoc -> Promise<string[]>

export async function getDocumentPageTexts(doc) {
  if (pageTextCache.has(doc)) {
    return pageTextCache.get(doc);
  }

  const promise = (async () => {
    const texts = [];
    const numPages = doc.numPages;
    for (let i = 1; i <= numPages; i++) {
      let page = null;
      try {
        page = await doc.getPage(i);
        const textContent = await page.getTextContent();
        const str = textContent.items.map((item) => item.str).join(' ');
        texts.push(str);
      } catch (e) {
        texts.push('');
      } finally {
        if (page) {
          try { page.cleanup(); } catch (_) {}
        }
      }
    }
    return texts;
  })();

  pageTextCache.set(doc, promise);
  return promise;
}

export function clearSearchCache(doc) {
  if (doc && pageTextCache.has(doc)) {
    pageTextCache.delete(doc);
  }
}

export async function searchPDF(doc, query) {
  if (!doc || !query || !query.trim()) return [];
  const q = query.trim().toLowerCase();
  const pageTexts = await getDocumentPageTexts(doc);
  const results = [];

  for (let pageIdx = 0; pageIdx < pageTexts.length; pageIdx++) {
    const text = pageTexts[pageIdx];
    if (!text) continue;

    const lowerText = text.toLowerCase();
    let count = 0;
    let firstIdx = -1;
    let idx = lowerText.indexOf(q);

    while (idx !== -1) {
      count++;
      if (firstIdx === -1) firstIdx = idx;
      idx = lowerText.indexOf(q, idx + q.length);
    }

    if (count > 0) {
      const start = Math.max(0, firstIdx - 45);
      const end = Math.min(text.length, firstIdx + q.length + 45);
      let snippet = text.slice(start, end).replace(/\s+/g, ' ');
      if (start > 0) snippet = '...' + snippet;
      if (end < text.length) snippet = snippet + '...';

      results.push({
        spineIndex: pageIdx,
        chapterTitle: `Page ${pageIdx + 1}`,
        snippet: snippet,
        matchCount: count,
      });
    }
  }

  return results;
}
