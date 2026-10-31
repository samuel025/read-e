<script>
  import { createEventDispatcher, tick } from 'svelte';

  // [{ value, label }]
  export let options = [];
  export let value = null;
  export let ariaLabel = '';
  export let align = 'left'; // which edge of the trigger the menu lines up with

  const dispatch = createEventDispatcher();
  const uid = Math.random().toString(36).slice(2, 8);

  let open = false;
  let rootEl;
  let triggerEl;
  let menuEl;
  let activeIndex = 0;

  $: selectedIndex = Math.max(0, options.findIndex((o) => o.value === value));
  $: selected = options[selectedIndex];

  async function openMenu() {
    if (open) return;
    open = true;
    activeIndex = selectedIndex;
    await tick();
    menuEl?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' });
  }

  function closeMenu(refocus = false) {
    if (!open) return;
    open = false;
    if (refocus) triggerEl?.focus();
  }

  function choose(index) {
    const opt = options[index];
    closeMenu(true);
    if (opt && opt.value !== value) {
      value = opt.value;
      dispatch('change', opt.value);
    }
  }

  function move(delta) {
    activeIndex = (activeIndex + delta + options.length) % options.length;
    tick().then(() => menuEl?.children[activeIndex]?.scrollIntoView({ block: 'nearest' }));
  }

  function onTriggerKeydown(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (!open) openMenu();
      else if (e.key === 'ArrowDown') move(1);
      else if (e.key === 'ArrowUp') move(-1);
      else choose(activeIndex);
    } else if (e.key === 'Escape' && open) {
      // preventDefault tells enclosing dialogs that this Escape was already used
      e.preventDefault();
      closeMenu(true);
    } else if (e.key === 'Home' && open) {
      e.preventDefault();
      activeIndex = 0;
    } else if (e.key === 'End' && open) {
      e.preventDefault();
      activeIndex = options.length - 1;
    } else if (e.key === 'Tab') {
      closeMenu();
    }
  }

  function onWindowPointerDown(e) {
    if (open && rootEl && !rootEl.contains(e.target)) closeMenu();
  }
</script>

<svelte:window on:pointerdown={onWindowPointerDown} />

<div class="dd" class:open bind:this={rootEl}>
  <button
    type="button"
    class="dd-trigger"
    bind:this={triggerEl}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-controls="dd-menu-{uid}"
    aria-label={ariaLabel ? `${ariaLabel}: ${selected?.label ?? ''}` : undefined}
    on:click={() => (open ? closeMenu() : openMenu())}
    on:keydown={onTriggerKeydown}
  >
    <span class="dd-label">{selected?.label ?? ''}</span>
    <svg class="dd-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <polyline points="6 9 12 15 18 9"></polyline>
    </svg>
  </button>

  {#if open}
    <div
      class="dd-menu"
      class:align-right={align === 'right'}
      id="dd-menu-{uid}"
      role="listbox"
      aria-label={ariaLabel}
      bind:this={menuEl}
    >
      {#each options as opt, i (opt.value)}
        <!-- Keyboard use goes through the trigger, which keeps focus -->
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <div
          class="dd-option"
          class:active={i === activeIndex}
          class:selected={opt.value === value}
          role="option"
          aria-selected={opt.value === value}
          tabindex="-1"
          on:click={() => choose(i)}
          on:mouseenter={() => (activeIndex = i)}
        >
          <span class="dd-option-label">{opt.label}</span>
          {#if opt.value === value}
            <svg class="dd-check" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .dd {
    position: relative;
    display: inline-block;
    --wails-draggable: no-drag;
  }

  .dd-trigger {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    height: 34px;
    padding: 0 10px 0 12px;
    min-width: 128px;
    background: var(--bg-hover);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--fg-primary);
    font-family: var(--font-sans);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition-property: background-color, border-color, color;
    transition-duration: var(--duration-fast);
    transition-timing-function: var(--ease-out);
  }

  .dd-trigger:hover {
    background: var(--bg-active);
  }

  .dd.open .dd-trigger {
    border-color: var(--accent);
    background: var(--bg-active);
  }

  .dd-trigger:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }

  .dd-label {
    white-space: nowrap;
  }

  .dd-chevron {
    flex-shrink: 0;
    color: var(--fg-tertiary);
    transition: transform var(--duration-fast) var(--ease-out);
  }

  .dd.open .dd-chevron {
    transform: rotate(180deg);
  }

  .dd-menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 50;
    min-width: 100%;
    max-height: min(360px, 60vh);
    overflow-y: auto;
    padding: 4px;
    background: var(--bg-elevated, var(--bg-primary));
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-sm);
    box-shadow: var(--shadow-lg);
    animation: ddIn 110ms var(--ease-out);
  }

  .dd-menu.align-right {
    left: auto;
    right: 0;
  }

  .dd-option {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 7px 10px;
    border-radius: 6px;
    font-family: var(--font-sans);
    font-size: 0.8125rem;
    color: var(--fg-secondary);
    cursor: pointer;
    white-space: nowrap;
    user-select: none;
    -webkit-user-select: none;
  }

  .dd-option.active {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  .dd-option.selected {
    color: var(--accent);
    font-weight: 600;
  }

  .dd-check {
    flex-shrink: 0;
  }

  /* Opacity and a small slide only: cheap without GPU compositing */
  @keyframes ddIn {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
