import { writable, derived } from 'svelte/store';

// ---- Library ----
export const library = writable([]);
export const libraryLoading = writable(false);
export const shelves = writable([]);
export const activeShelfId = writable('all'); // 'all' | 'reading' | 'finished' | shelfID

// ---- Reader state ----
export const currentBook = writable(null);
export const currentBookId = writable(null);
export const currentChapter = writable('');
export const currentSpineIndex = writable(0);
export const toc = writable([]);
export const spineCount = writable(0);
export const readingStats = writable({ words: 0, minutesLeft: 0 });
export const chapterPageInfo = writable({
  currentPage: 1,
  totalPages: 1,
  pagesLeft: 0,
  chapterTitle: '',
  percentInChapter: 0,
});
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

// ---- Theme ----
// 'auto' follows the operating system's light/dark preference.
export const systemDark = writable(true);

if (typeof window !== 'undefined' && window.matchMedia) {
  const query = window.matchMedia('(prefers-color-scheme: dark)');
  systemDark.set(query.matches);
  const onChange = (e) => systemDark.set(e.matches);
  if (query.addEventListener) query.addEventListener('change', onChange);
  else if (query.addListener) query.addListener(onChange);
}

export function resolveTheme(theme, dark) {
  return theme === 'auto' ? (dark ? 'dark' : 'light') : theme;
}

// The theme actually applied (never 'auto')
export const activeTheme = derived([settings, systemDark], ([$s, $dark]) => resolveTheme($s?.theme || 'dark', $dark));

activeTheme.subscribe((theme) => {
  if (typeof document !== 'undefined' && theme) {
    document.documentElement.setAttribute('data-theme', theme);
  }
});
