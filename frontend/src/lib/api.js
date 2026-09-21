/*
  API wrapper around Wails-generated Go bindings.
  In dev mode (no Wails runtime), falls back to mock data.
*/

function getGoBinding() {
  if (
    typeof window !== 'undefined' &&
    window.go &&
    window.go.main &&
    window.go.main.App
  ) {
    return window.go.main.App;
  }
  return null;
}

export async function scanLibrary(folderPath) {
  const go = getGoBinding();
  if (!go) return [];
  return go.ScanLibrary(folderPath);
}

export async function addBook(filePath) {
  const go = getGoBinding();
  if (!go) return [];
  return go.AddBook(filePath);
}

export async function getLibrary() {
  const go = getGoBinding();
  if (!go) return [];
  return go.GetLibrary();
}

export async function removeBook(bookID) {
  const go = getGoBinding();
  if (!go) return;
  return go.RemoveBook(bookID);
}

export async function updateBookCover(bookID, coverBase64) {
  const go = getGoBinding();
  if (!go) return;
  return go.UpdateBookCover(bookID, coverBase64);
}

export async function logReadingSession(date, durationSecs, pagesTurned) {
  const go = getGoBinding();
  if (!go) return;
  return go.LogReadingSession(date, durationSecs, pagesTurned);
}

export async function markBookFinished(bookID, finished) {
  const go = getGoBinding();
  if (!go) return;
  return go.MarkBookFinished(bookID, finished);
}

export async function getReadingInsights() {
  const go = getGoBinding();
  if (!go) return { total_hours: 0, finished_books: 0, current_streak: 0, longest_streak: 0, daily_data: [] };
  return go.GetReadingInsights();
}

export async function openBook(bookID) {
  const go = getGoBinding();
  if (!go) return {};
  return go.OpenBook(bookID);
}

export async function getTOC(bookID) {
  const go = getGoBinding();
  if (!go) return [];
  return go.GetTOC(bookID);
}

export async function getChapter(bookID, spineIndex) {
  const go = getGoBinding();
  if (!go) return '<p>No Wails runtime available.</p>';
  return go.GetChapter(bookID, spineIndex);
}

export async function saveProgress(bookID, spineIndex, scrollOffset) {
  const go = getGoBinding();
  if (!go) return;
  return go.SaveProgress(bookID, spineIndex, scrollOffset);
}

export async function getProgress(bookID) {
  const go = getGoBinding();
  if (!go) return { bookId: bookID, spineIndex: 0, scrollOffset: 0 };
  return go.GetProgress(bookID);
}

export async function saveSettings(settings) {
  const go = getGoBinding();
  if (!go) return;
  return go.SaveSettings(settings);
}

export async function getSettings() {
  const go = getGoBinding();
  if (!go) return { theme: 'dark', fontSize: 1.0, fontFamily: "'Inter', system-ui, sans-serif" };
  return go.GetSettings();
}

export async function selectFolder() {
  const go = getGoBinding();
  if (!go) return '';
  return go.SelectFolder();
}

export async function selectFile() {
  const go = getGoBinding();
  if (!go) return '';
  return go.SelectFile();
}

export async function addHighlight(highlight) {
  const go = getGoBinding();
  if (!go) return;
  return go.AddHighlight(highlight);
}

export async function getHighlights(bookID) {
  const go = getGoBinding();
  if (!go) return [];
  return go.GetHighlights(bookID);
}

export async function deleteHighlight(id) {
  const go = getGoBinding();
  if (!go) return;
  return go.DeleteHighlight(id);
}

export async function updateHighlightNote(id, note) {
  const go = getGoBinding();
  if (!go) return;
  return go.UpdateHighlightNote(id, note);
}

export async function saveBookmark(bookmark) {
  const go = getGoBinding();
  if (!go) return;
  return go.SaveBookmark(bookmark);
}

export async function getBookmarks(bookID) {
  const go = getGoBinding();
  if (!go) return [];
  return go.GetBookmarks(bookID);
}

export async function deleteBookmark(id) {
  const go = getGoBinding();
  if (!go) return;
  return go.DeleteBookmark(id);
}

export async function searchBook(bookID, query) {
  const go = getGoBinding();
  if (!go) return [];
  return go.SearchBook(bookID, query);
}

export async function getChapterWordCount(bookID, spineIndex) {
  const go = getGoBinding();
  if (!go) return 0;
  return go.GetChapterWordCount(bookID, spineIndex);
}

export async function getPDFData(bookID) {
  const go = getGoBinding();
  if (!go) return null;
  const res = await go.GetPDFData(bookID);
  if (!res) return null;
  if (typeof res === 'string') {
    const binaryString = atob(res);
    const len = binaryString.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }
    return bytes;
  }
  if (Array.isArray(res)) {
    return new Uint8Array(res);
  }
  return res;
}

