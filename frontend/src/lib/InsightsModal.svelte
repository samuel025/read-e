<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { getReadingInsights } from './api.js';

  const dispatch = createEventDispatcher();
  let loading = true;
  let insights = null;

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

  // Activity chart configuration
  $: dailyData = insights?.daily_data || [];
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
        duration: match ? match.duration_seconds : 0,
        pages: match ? match.pages_turned : 0
      });
    }
    return days;
  }

  function formatHours(hours) {
    if (hours < 0.1) return (hours * 60).toFixed(0) + ' min';
    return hours.toFixed(1) + ' hrs';
  }

  function getLevel(durationSecs) {
    if (durationSecs === 0) return 'level-0';
    if (durationSecs < 600) return 'level-1'; // < 10 mins
    if (durationSecs < 1800) return 'level-2'; // < 30 mins
    if (durationSecs < 3600) return 'level-3'; // < 1 hour
    return 'level-4'; // > 1 hour
  }
</script>

<div class="modal-backdrop" on:click={close}>
  <div class="modal-content" on:click|stopPropagation>
    <div class="modal-header">
      <h2>Reading Insights</h2>
      <button class="close-btn" on:click={close}>
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
    </div>

    {#if loading}
      <div class="loading-state">Loading your stats...</div>
    {:else if insights}
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-value">{formatHours(insights.total_hours)}</div>
          <div class="stat-label">Total Read Time</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{insights.finished_books}</div>
          <div class="stat-label">Books Finished</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{insights.current_streak}</div>
          <div class="stat-label">Day Streak</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{insights.longest_streak}</div>
          <div class="stat-label">Longest Streak</div>
        </div>
      </div>

      <div class="activity-section">
        <h3>Last 30 Days Activity</h3>
        <div class="activity-chart">
          {#each last30Days as day}
            <div 
              class="activity-bar {getLevel(day.duration)}"
              title="{day.date}: {Math.round(day.duration/60)} mins read"
            ></div>
          {/each}
        </div>
        <div class="activity-legend">
          <span>Less</span>
          <div class="activity-bar level-0"></div>
          <div class="activity-bar level-1"></div>
          <div class="activity-bar level-2"></div>
          <div class="activity-bar level-3"></div>
          <div class="activity-bar level-4"></div>
          <span>More</span>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal-content {
    background: var(--bg-primary, #ffffff);
    color: var(--text-primary, #1e293b);
    border-radius: 12px;
    width: 90%;
    max-width: 600px;
    padding: 24px;
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
  }

  .modal-header h2 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
  }

  .close-btn {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-secondary, #64748b);
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
    gap: 16px;
    margin-bottom: 32px;
  }

  .stat-card {
    background: var(--bg-secondary, #f1f5f9);
    padding: 16px;
    border-radius: 8px;
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .stat-value {
    font-size: 2rem;
    font-weight: 800;
    color: var(--accent-color, #3b82f6);
  }

  .stat-label {
    font-size: 0.875rem;
    color: var(--text-secondary, #64748b);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .activity-section h3 {
    font-size: 1.1rem;
    margin-bottom: 16px;
  }

  .activity-chart {
    display: flex;
    gap: 4px;
    height: 60px;
    align-items: flex-end;
    margin-bottom: 12px;
  }

  .activity-bar {
    flex: 1;
    min-width: 8px;
    border-radius: 2px;
    background: var(--bg-secondary, #f1f5f9);
    height: 100%;
  }

  .activity-bar.level-0 { height: 10%; background: var(--bg-secondary, #f1f5f9); }
  .activity-bar.level-1 { height: 30%; background: #93c5fd; }
  .activity-bar.level-2 { height: 50%; background: #60a5fa; }
  .activity-bar.level-3 { height: 75%; background: #3b82f6; }
  .activity-bar.level-4 { height: 100%; background: #2563eb; }

  .activity-legend {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--text-secondary, #64748b);
  }
  
  .activity-legend .activity-bar {
    width: 12px;
    height: 12px;
    flex: none;
    border-radius: 2px;
  }
</style>
