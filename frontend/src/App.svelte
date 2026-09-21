<script>
  import { view } from './stores/app.js';
  import { getLibrary, getSettings } from './lib/api.js';
  import { library, settings } from './stores/app.js';
  import LibraryView from './lib/LibraryView.svelte';
  import ReaderView from './lib/ReaderView.svelte';
  import { onMount } from 'svelte';

  onMount(async () => {
    // Load saved settings
    const savedSettings = await getSettings();
    if (savedSettings) {
      settings.set(savedSettings);
    }

    // Load library
    const books = await getLibrary();
    if (books) {
      library.set(books);
    }
  });
</script>

{#if $view === 'library'}
  <LibraryView />
{:else}
  <ReaderView />
{/if}

<style>
  :global(#app) {
    height: 100vh;
    overflow: hidden;
  }
</style>
