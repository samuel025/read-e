<script>
  export let item;
  export let depth = 0;
  export let activeKey = '';
  export let getEntryKey = () => '';
  export let onNavigate = () => {};

  let isExpanded = true;

  function toggleExpand(e) {
    e.stopPropagation();
    isExpanded = !isExpanded;
  }
</script>

<li class="toc-item">
  <div
    class="toc-entry-row"
    class:active={activeKey === getEntryKey(item)}
    style="padding-left: {8 + depth * 14}px;"
  >
    {#if item.children && item.children.length > 0}
      <button
        type="button"
        class="toc-chevron-btn"
        class:collapsed={!isExpanded}
        on:click={toggleExpand}
        title={isExpanded ? 'Collapse section' : 'Expand section'}
        aria-label={isExpanded ? 'Collapse section' : 'Expand section'}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </button>
    {:else if depth > 0}
      <span class="toc-bullet" aria-hidden="true"></span>
    {:else}
      <span class="toc-spacer" aria-hidden="true"></span>
    {/if}

    <button
      type="button"
      class="toc-entry-btn"
      on:click={() => onNavigate(item)}
      title={item.title}
    >
      <span class="toc-entry-title">{item.title}</span>
    </button>
  </div>

  {#if item.children && item.children.length > 0 && isExpanded}
    <ul class="toc-nested-list">
      {#each item.children as child}
        <svelte:self
          item={child}
          depth={depth + 1}
          {activeKey}
          {getEntryKey}
          {onNavigate}
        />
      {/each}
    </ul>
  {/if}
</li>

<style>
  .toc-item {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .toc-nested-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .toc-entry-row {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    margin: 1px 0;
    border-radius: var(--radius-sm);
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
    user-select: none;
  }

  .toc-entry-row:hover {
    background: var(--bg-hover);
  }

  .toc-entry-row.active {
    background: var(--accent-subtle);
  }

  .toc-chevron-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border: none;
    background: transparent;
    padding: 0;
    border-radius: 4px;
    color: var(--fg-tertiary);
    cursor: pointer;
    flex-shrink: 0;
    transition: transform var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out),
                background var(--duration-fast) var(--ease-out);
  }

  .toc-chevron-btn:hover {
    color: var(--fg-primary);
    background: var(--bg-card);
  }

  .toc-chevron-btn.collapsed {
    transform: rotate(-90deg);
  }

  .toc-bullet {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--border-subtle);
    flex-shrink: 0;
    margin: 0 8px;
  }

  .toc-entry-row.active .toc-bullet {
    background: var(--accent);
  }

  .toc-spacer {
    width: 8px;
    flex-shrink: 0;
  }

  .toc-entry-btn {
    flex: 1;
    display: flex;
    align-items: center;
    border: none;
    background: transparent;
    text-align: left;
    padding: 6px 8px 6px 2px;
    color: var(--fg-secondary);
    font-family: var(--font-sans);
    font-size: 0.8125rem;
    line-height: 1.4;
    cursor: pointer;
    overflow: hidden;
  }

  .toc-entry-row:hover .toc-entry-btn {
    color: var(--fg-primary);
  }

  .toc-entry-row.active .toc-entry-btn {
    color: var(--accent);
    font-weight: 600;
  }

  .toc-entry-title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
