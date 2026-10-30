<script>
  import { view, settingsOpen, searchOpen, library, settings } from './stores/app.js';
  import { getLibrary, getSettings, getPendingOpen } from './lib/api.js';
  import { openBookById } from './lib/openBook.js';
  import LibraryView from './lib/LibraryView.svelte';
  import ReaderView from './lib/ReaderView.svelte';
  import SettingsPanel from './lib/SettingsPanel.svelte';
  import SearchModal from './lib/SearchModal.svelte';
  import { onMount, onDestroy } from 'svelte';

  onMount(async () => {
    // Load saved settings
    const savedSettings = await getSettings();
    if (savedSettings) {
      settings.set(savedSettings);
    }

    // Load library
    const books = await getLibrary();
    if (books && Array.isArray(books)) {
      library.set(books);
    }

    // Launched with a book (double-click in the file manager / command line)
    const pending = await getPendingOpen();
    if (pending) openBookById(pending);

    // A later launch while this window is already running
    offOpenBook = window.runtime?.EventsOn?.('open-book', async (id) => {
      const books = await getLibrary();
      if (Array.isArray(books)) library.set(books);
      openBookById(id);
    });
  });

  let offOpenBook = null;
  onDestroy(() => { if (typeof offOpenBook === 'function') offOpenBook(); });
</script>

{#if $view === 'library'}
  <LibraryView />
{:else}
  <ReaderView />
{/if}

{#if $settingsOpen}
  <SettingsPanel />
{/if}

{#if $searchOpen}
  <SearchModal />
{/if}

<style>
  :global(#app) {
    height: 100vh;
    overflow: hidden;
  }
</style>
