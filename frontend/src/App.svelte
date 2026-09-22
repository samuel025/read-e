<script>
  import { view, settingsOpen, searchOpen, library, settings } from './stores/app.js';
  import { getLibrary, getSettings } from './lib/api.js';
  import LibraryView from './lib/LibraryView.svelte';
  import ReaderView from './lib/ReaderView.svelte';
  import SettingsPanel from './lib/SettingsPanel.svelte';
  import SearchModal from './lib/SearchModal.svelte';
  import { onMount } from 'svelte';

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
  });
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
