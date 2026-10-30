import { get } from 'svelte/store';
import { tick } from 'svelte';
import { openBook, getTOC, getProgress, getChapter } from './api.js';
import {
  currentBook, currentBookId, currentChapter,
  currentSpineIndex, spineCount, toc, view
} from '../stores/app.js';

// Opens a book in the reader at its saved position. Shared by library cards,
// "continue reading" and files opened from the file manager.
export async function openBookById(bookId) {
  // Switching books from inside the reader: leave it first so it saves progress
  // and its panes are torn down before the stores change under them.
  if (get(view) === 'reader') {
    view.set('library');
    await tick();
  }

  currentBookId.set(bookId);

  try {
    const info = await openBook(bookId);
    currentBook.set(info);

    if (info.format === 'pdf') {
      view.set('reader');
      return;
    }

    spineCount.set(info.spineCount);

    const tocData = await getTOC(bookId);
    toc.set(tocData || []);

    const progress = await getProgress(bookId);
    const startIndex = progress?.spineIndex || 0;
    currentSpineIndex.set(startIndex);

    const html = await getChapter(bookId, startIndex);
    currentChapter.set(html);

    view.set('reader');
  } catch (err) {
    console.error('Failed to open book:', err);
  }
}
