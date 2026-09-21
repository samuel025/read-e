import { writable } from 'svelte/store';

// ---- Library ----
export const library = writable([]);
export const libraryLoading = writable(false);

// ---- Reader state ----
export const currentBook = writable(null);
export const currentBookId = writable(null);
export const currentChapter = writable('');
export const currentSpineIndex = writable(0);
export const toc = writable([]);
export const spineCount = writable(0);
export const readingStats = writable({ words: 0, minutesLeft: 0 });
export const pdfDoc = writable(null);
export const pdfZoom = writable(100); // percentage: 100, 125, or -1 for fit-width

// ---- UI state ----
export const view = writable('library'); // 'library' | 'reader'
export const tocOpen = writable(true);
export const settingsOpen = writable(false);
export const searchOpen = writable(false);
export const activeSidebarTab = writable('toc'); // 'toc' | 'highlights' | 'bookmarks'
export const highlights = writable([]); // Highlight[]
export const bookmarks = writable([]);   // Bookmark[]

// ---- Settings ----
export const settings = writable({
  theme: 'dark',
  fontSize: 1.0,
  fontFamily: "'Inter', system-ui, sans-serif",
  maxWidth: 780,
  lineHeight: 1.7,
  textAlign: 'left',
});

settings.subscribe((s) => {
  if (typeof document !== 'undefined' && s?.theme) {
    document.documentElement.setAttribute('data-theme', s.theme);
  }
});
