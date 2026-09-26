<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { getReadingInsights } from './api.js';

  const dispatch = createEventDispatcher();
  let loading = true;
  let insights = null;
  let hoveredDay = null;

  onMount(async () => {
    try {
      insights = await getReadingInsights();
    } catch (e) {
      console.error('Failed to load insights:', e);
    } finally {
      loading = false;
    }
  });

  function close() {
    dispatch('close');
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      close();
    }
  }

  // Activity data processing
  $: dailyData = (insights && insights.daily_data) || [];
  $: last30Days = generateLast30Days(dailyData);

  function generateLast30Days(data) {
    const days = [];
    const today = new Date();
    for (let i = 29; i >= 0; i--) {
      const d = new Date(today);
      d.setDate(today.getDate() - i);
      const dateStr = d.toISOString().split('T')[0];
      const match = data.find(x => x.date === dateStr);
      days.push({
        date: dateStr,
        dayName: d.toLocaleDateString(undefined, { weekday: 'short' }),
        formattedDate: d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
        duration: match ? match.duration_seconds : 0,
        pages: match ? match.pages_turned : 0
      });
    }
    return days;
  }

  // Derived statistics
  $: activeDaysCount = last30Days.filter(d => d.duration > 0).length;
  $: totalSeconds30 = last30Days.reduce((acc, d) => acc + d.duration, 0);
  $: totalPages30 = last30Days.reduce((acc, d) => acc + d.pages, 0);
  $: avgMinsActiveDay = activeDaysCount > 0 ? Math.round((totalSeconds30 / 60) / activeDaysCount) : 0;
  $: maxDuration = Math.max(...last30Days.map(d => d.duration), 3600);

  function formatTime(seconds) {
    if (!seconds || seconds <= 0) return '0m';
    const hrs = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    if (hrs > 0) {
      return mins > 0 ? `${hrs}h ${mins}m` : `${hrs}h`;
    }
    return `${mins}m`;
  }

  function formatTotalHours(hours) {
    if (!hours || hours <= 0) return '0h';
    const totalSecs = Math.round(hours * 3600);
    return formatTime(totalSecs);
  }

  function getBarHeight(durationSecs) {
    if (durationSecs <= 0) return 6;
    const pct = Math.max(14, Math.round((durationSecs / maxDuration) * 100));
    return Math.min(100, pct);
  }

  function getLevel(durationSecs) {
    if (durationSecs === 0) return 'level-0';
    if (durationSecs < 600) return 'level-1';   // < 10 mins
    if (durationSecs < 1800) return 'level-2';  // < 30 mins
    if (durationSecs < 3600) return 'level-3';  // < 60 mins
    return 'level-4';                          // 60+ mins
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div
  class="modal-backdrop"
  on:click={close}
  role="dialog"
  aria-modal="true"
  aria-labelledby="insights-title"
  tabindex="-1"
>
  <div class="modal-box" on:click|stopPropagation role="document">
    <!-- Header -->
    <div class="modal-header">
      <div class="header-left">
        <div class="header-icon-wrap">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 20V10M18 20V4M6 20v-4" />
          </svg>
        </div>
        <div class="header-titles">
          <h2 id="insights-title">Reading Insights</h2>
          <span class="header-subtitle">Your habits, streaks, and milestones</span>
        </div>
      </div>
      <button class="close-btn" on:click={close} aria-label="Close modal">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 6 6 18"/><path d="m6 6 12 12"/>
        </svg>
      </button>
    </div>

    <!-- Content -->
    <div class="modal-body">
      {#if loading}
        <div class="loading-state">
          <div class="spinner"></div>
          <p>Calculating your reading statistics...</p>
        </div>
      {:else if insights}
        <!-- 4 Minimalist Stat Cards Grid -->
        <div class="stats-grid">
          <!-- Total Time -->
          <div class="stat-card">
            <div class="stat-top">
              <span class="stat-label">Total Time</span>
              <div class="stat-icon-wrap">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <polyline points="12 6 12 12 16 14"/>
                </svg>
              </div>
            </div>
            <div class="stat-value">{formatTotalHours(insights.total_hours)}</div>
            <div class="stat-sub">Across all books</div>
          </div>

          <!-- Finished Books -->
          <div class="stat-card">
            <div class="stat-top">
              <span class="stat-label">Finished</span>
              <div class="stat-icon-wrap">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>
                  <polyline points="9 11 12 14 17 9"/>
                </svg>
              </div>
            </div>
            <div class="stat-value">{insights.finished_books || 0}</div>
            <div class="stat-sub">Completed titles</div>
          </div>

          <!-- Current Streak -->
          <div class="stat-card">
            <div class="stat-top">
              <span class="stat-label">Streak</span>
              <div class="stat-icon-wrap">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z"/>
                </svg>
              </div>
            </div>
            <div class="stat-value">
              {insights.current_streak || 0}
              <span class="stat-unit">{insights.current_streak === 1 ? 'day' : 'days'}</span>
            </div>
            <div class="stat-sub">Current momentum</div>
          </div>

          <!-- Longest Streak -->
          <div class="stat-card">
            <div class="stat-top">
              <span class="stat-label">Best Streak</span>
              <div class="stat-icon-wrap">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
                </svg>
              </div>
            </div>
            <div class="stat-value">
              {insights.longest_streak || 0}
              <span class="stat-unit">{insights.longest_streak === 1 ? 'day' : 'days'}</span>
            </div>
            <div class="stat-sub">Personal record</div>
          </div>
        </div>

        <!-- 30 Days Activity Timeline & Chart (Shades of Blue) -->
        <div class="activity-card">
          <div class="activity-header">
            <div class="activity-title-wrap">
              <span class="activity-card-title">30-Day Activity</span>
              <span class="activity-meta-badge">
                {activeDaysCount} of 30 days active
              </span>
            </div>
            {#if avgMinsActiveDay > 0}
              <span class="activity-avg">
                Avg: <strong>{avgMinsActiveDay}m</strong> / active day
              </span>
            {/if}
          </div>

          <!-- Interactive Bar Timeline -->
          <div class="chart-timeline-container">
            {#each last30Days as day, i}
              {@const isHovered = hoveredDay && hoveredDay.date === day.date}
              <div
                class="timeline-col"
                on:mouseenter={() => hoveredDay = day}
                on:mouseleave={() => hoveredDay = null}
                role="presentation"
              >
                <!-- Tooltip on hover -->
                {#if isHovered}
                  <div class="chart-tooltip">
                    <span class="tooltip-date">{day.dayName}, {day.formattedDate}</span>
                    <span class="tooltip-duration">{formatTime(day.duration)}</span>
                    {#if day.pages > 0}
                      <span class="tooltip-pages">{day.pages} {day.pages === 1 ? 'page' : 'pages'}</span>
                    {/if}
                  </div>
                {/if}

                <div class="bar-track">
                  <div
                    class="bar-fill {getLevel(day.duration)}"
                    style="height: {getBarHeight(day.duration)}%;"
                  ></div>
                </div>

                <!-- Show date labels at intervals -->
                {#if i === 0 || i === 9 || i === 19 || i === 29}
                  <span class="timeline-label">{day.formattedDate}</span>
                {/if}
              </div>
            {/each}
          </div>

          <!-- Blue Heatmap Legend (Strictly shades of blue, no brown) -->
          <div class="activity-footer">
            <div class="activity-summary-stats">
              <span>{totalPages30} pages read in past 30 days</span>
            </div>
            <div class="activity-legend">
              <span class="legend-text">Less</span>
              <span class="legend-swatch level-0" title="0m"></span>
              <span class="legend-swatch level-1" title="1-10m"></span>
              <span class="legend-swatch level-2" title="10-30m"></span>
              <span class="legend-swatch level-3" title="30-60m"></span>
              <span class="legend-swatch level-4" title="60m+"></span>
              <span class="legend-text">More</span>
            </div>
          </div>
        </div>
      {/if}
    </div>

    <!-- Footer -->
    <div class="modal-footer">
      <button type="button" class="btn btn-secondary" on:click={close}>
        Close
      </button>
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: var(--space-md);
    animation: fadeIn var(--duration-fast) var(--ease-out);
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .modal-box {
    background: var(--bg-card);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 640px;
    box-shadow: var(--shadow-xl);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slideUp var(--duration-normal) var(--ease-out);
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(12px) scale(0.985);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  /* Header */
  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-md) var(--space-lg);
    border-bottom: 1px solid var(--border-subtle);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: var(--space-md);
  }

  .header-icon-wrap {
    width: 34px;
    height: 34px;
    border-radius: var(--radius-sm);
    background: var(--bg-hover);
    color: var(--fg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .header-titles {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .modal-header h2 {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--fg-primary);
    margin: 0;
  }

  .header-subtitle {
    font-size: 0.8125rem;
    color: var(--fg-tertiary);
  }

  .close-btn {
    background: transparent;
    border: none;
    color: var(--fg-tertiary);
    cursor: pointer;
    padding: 6px;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background var(--duration-fast) var(--ease-out),
                color var(--duration-fast) var(--ease-out);
  }

  .close-btn:hover {
    background: var(--bg-hover);
    color: var(--fg-primary);
  }

  /* Body */
  .modal-body {
    padding: var(--space-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    overflow-y: auto;
    max-height: 75vh;
  }

  /* Stats Grid - Minimalist & Monochromatic */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-md);
  }

  @media (max-width: 600px) {
    .stats-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  .stat-card {
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    padding: var(--space-md);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    transition: transform var(--duration-fast) var(--ease-out),
                border-color var(--duration-fast) var(--ease-out);
  }

  .stat-card:hover {
    transform: translateY(-2px);
    border-color: var(--border-medium);
  }

  .stat-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-sm);
  }

  .stat-label {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fg-tertiary);
  }

  .stat-icon-wrap {
    width: 26px;
    height: 26px;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-hover);
    color: var(--fg-secondary);
  }

  .stat-value {
    font-size: 1.4375rem;
    font-weight: 700;
    color: var(--fg-primary);
    line-height: 1.2;
    letter-spacing: -0.01em;
    margin-bottom: 2px;
  }

  .stat-unit {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--fg-secondary);
  }

  .stat-sub {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
  }

  /* Activity Card */
  .activity-card {
    background: var(--bg-primary);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-md);
    padding: var(--space-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  .activity-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-sm);
  }

  .activity-title-wrap {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }

  .activity-card-title {
    font-size: 0.875rem;
    font-weight: 700;
    color: var(--fg-primary);
  }

  .activity-meta-badge {
    font-size: 0.6875rem;
    font-weight: 600;
    background: var(--bg-hover);
    color: var(--fg-secondary);
    padding: 2px 8px;
    border-radius: 100px;
  }

  .activity-avg {
    font-size: 0.8125rem;
    color: var(--fg-secondary);
  }

  .activity-avg strong {
    color: var(--fg-primary);
  }

  /* Chart Timeline */
  .chart-timeline-container {
    display: flex;
    align-items: flex-end;
    gap: 4px;
    height: 100px;
    padding-top: 24px;
    padding-bottom: 18px;
    position: relative;
    border-bottom: 1px solid var(--border-subtle);
  }

  .timeline-col {
    flex: 1;
    height: 100%;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    align-items: center;
    position: relative;
    cursor: pointer;
  }

  .bar-track {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: flex-end;
    justify-content: center;
  }

  .bar-fill {
    width: 100%;
    max-width: 14px;
    border-radius: 3px;
    transition: height var(--duration-normal) var(--ease-out),
                filter var(--duration-fast) var(--ease-out);
  }

  .timeline-col:hover .bar-fill {
    filter: brightness(1.2);
  }

  /* Strict Shades of Blue (No brown across Dark, Light, or Sepia) */
  .bar-fill.level-0 {
    background: rgba(148, 163, 184, 0.16);
  }

  .bar-fill.level-1 {
    background: #93c5fd; /* Soft Ice Blue */
  }

  .bar-fill.level-2 {
    background: #60a5fa; /* Medium Sky Blue */
  }

  .bar-fill.level-3 {
    background: #3b82f6; /* Vibrant Azure Blue */
  }

  .bar-fill.level-4 {
    background: #1d4ed8; /* Deep Royal Blue */
  }

  .timeline-label {
    position: absolute;
    bottom: -16px;
    font-size: 0.625rem;
    color: var(--fg-tertiary);
    white-space: nowrap;
    pointer-events: none;
  }

  /* Tooltip */
  .chart-tooltip {
    position: absolute;
    top: -24px;
    transform: translateY(-50%);
    background: var(--bg-card);
    border: 1px solid var(--border-medium);
    border-radius: var(--radius-sm);
    padding: 6px 10px;
    box-shadow: var(--shadow-lg);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    white-space: nowrap;
    z-index: 20;
    pointer-events: none;
    animation: fadeIn 100ms var(--ease-out);
  }

  .tooltip-date {
    font-size: 0.6875rem;
    color: var(--fg-tertiary);
    font-weight: 500;
  }

  .tooltip-duration {
    font-size: 0.8125rem;
    font-weight: 700;
    color: #3b82f6;
  }

  .tooltip-pages {
    font-size: 0.6875rem;
    color: var(--fg-secondary);
  }

  /* Activity Footer & Blue Legend */
  .activity-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-sm);
    padding-top: var(--space-xs);
  }

  .activity-summary-stats {
    font-size: 0.75rem;
    color: var(--fg-tertiary);
  }

  .activity-legend {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .legend-text {
    font-size: 0.6875rem;
    color: var(--fg-tertiary);
  }

  .legend-swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }

  /* Legend swatches explicitly in pure blue shades */
  .legend-swatch.level-0 { background: rgba(148, 163, 184, 0.2); }
  .legend-swatch.level-1 { background: #93c5fd; }
  .legend-swatch.level-2 { background: #60a5fa; }
  .legend-swatch.level-3 { background: #3b82f6; }
  .legend-swatch.level-4 { background: #1d4ed8; }

  /* Loading State */
  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-md);
    padding: var(--space-2xl) 0;
    color: var(--fg-secondary);
    font-size: 0.875rem;
  }

  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid var(--border-subtle);
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Modal Footer */
  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: var(--space-md) var(--space-lg);
    border-top: 1px solid var(--border-subtle);
  }
</style>
