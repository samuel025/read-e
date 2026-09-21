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
