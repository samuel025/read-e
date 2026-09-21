import { writable } from 'svelte/store';

// ---- Library ----
export const library = writable([]);
export const libraryLoading = writable(false);

// ---- Reader state ----
export const currentBook = writable(null);     // BookInfo or null
export const currentBookId = writable(null);
export const currentChapter = writable('');
export const currentSpineIndex = writable(0);
export const toc = writable([]);
export const spineCount = writable(0);

// ---- UI state ----
export const view = writable('library'); // 'library' | 'reader'
export const tocOpen = writable(true);
export const settingsOpen = writable(false);
export const activeSidebarTab = writable('toc'); // 'toc' | 'highlights'
export const highlights = writable([]); // Highlight[]

// ---- Settings ----
export const settings = writable({
  theme: 'dark',
  fontSize: 1.0,
  fontFamily: "'Inter', system-ui, sans-serif",
});

// Apply theme reactively
settings.subscribe((s) => {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', s.theme);
  }
});
