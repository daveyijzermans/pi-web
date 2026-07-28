<script>
  import { onMount } from 'svelte';
  import { icon, FolderGit2, Search } from '../../shared/icons.js';
  import { t } from '../../shared/i18n.js';
  import { handleNavClick } from '../../shared/navigation.js';
  import {
    defaultFetchSessions,
    formatRelativeTime,
    groupSessionsByDate,
    normalizeSession,
    sessionModelLabel,
    sessionSearchText,
    sessionsCountLabel,
  } from '../../index/sessions.js';
  import { prefetchSession } from '../../routes/session-prefetch.js';
  import { getSpinnerConfig } from '../../session/live/chat-preview.js';
  import { sessionRuntime } from '../../session/session-runtime.js';

  let {
    cwd = '',
    currentSessionId = '',
    fetchSessions = defaultFetchSessions,
    runningSessionIds = null,
  } = $props();

  let sessions = $state([]);
  let query = $state('');
  let loading = $state(true);
  let error = $state('');
  let now = $state(Date.now());
  let spinnerChar = $state('');
  let spinnerStyle = $state('');

  const projectName = $derived(cwd.split(/[\\/]/).filter(Boolean).at(-1) || cwd);
  const filteredSessions = $derived.by(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) return sessions;
    return sessions.filter((session) =>
      sessionSearchText(session).toLowerCase().includes(normalizedQuery),
    );
  });
  const groupedSessions = $derived(groupSessionsByDate(filteredSessions, now));

  const dateBucketKeys = {
    today: 'index.dateToday',
    yesterday: 'index.dateYesterday',
    previous7days: 'index.datePrevious7Days',
    previous30days: 'index.datePrevious30Days',
    older: 'index.dateOlder',
  };

  function startPrefetch(sessionId) {
    if (sessionId) prefetchSession(sessionId);
  }

  function isRunning(sessionId) {
    return !!runningSessionIds?.has(sessionId);
  }

  $effect(() => {
    if (!runningSessionIds?.size) {
      spinnerChar = '';
      spinnerStyle = '';
      return;
    }

    const config = getSpinnerConfig(window);
    let frame = 0;
    spinnerChar = config.frames[0] || '';
    spinnerStyle = `font-family:${config.fontFamily};width:${config.width}`;
    const timer = window.setInterval(() => {
      frame = (frame + 1) % config.frames.length;
      spinnerChar = config.frames[frame] || '';
    }, config.interval);
    return () => window.clearInterval(timer);
  });

  function onSessionClick(event, href) {
    handleNavClick(event, href);
    if (sessionRuntime.layout?.isMobileLayout?.()) sessionRuntime.layout?.closeSidebar?.();
  }

  onMount(() => {
    let active = true;
    const timer = setInterval(() => {
      now = Date.now();
    }, 60000);

    if (!cwd) {
      loading = false;
      return () => clearInterval(timer);
    }

    fetchSessions({ project: cwd })
      .then((response) => {
        if (!active) return;
        sessions = (response.sessions || []).map(normalizeSession);
      })
      .catch((err) => {
        if (!active) return;
        error = err?.message || t('session.sessionsLoadFailed');
      })
      .finally(() => {
        if (active) loading = false;
      });

    return () => {
      active = false;
      clearInterval(timer);
    };
  });
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG -->

<div class="sidebar-session-controls">
  {#if cwd}
    <div class="sidebar-project" title={cwd}>
      <span class="sidebar-project-icon">{@html icon(FolderGit2, { size: 15 })}</span>
      <span class="sidebar-project-copy">
        <span class="sidebar-project-label">{t('session.currentProject')}</span>
        <span class="sidebar-project-name">{projectName}</span>
      </span>
      {#if !loading && !error}
        <span
          class="sidebar-project-count"
          title={sessionsCountLabel(sessions.length)}
          aria-label={sessionsCountLabel(sessions.length)}>{sessions.length}</span
        >
      {/if}
    </div>
  {/if}
  <label class="sidebar-search-shell">
    <span class="sidebar-search-icon">{@html icon(Search, { size: 13 })}</span>
    <input
      type="search"
      class="sidebar-search"
      bind:value={query}
      placeholder={t('session.searchProjectSessions')}
      aria-label={t('session.searchProjectSessions')}
    />
  </label>
</div>

<div class="sidebar-session-list" aria-live="polite">
  {#if loading}
    <div class="sidebar-session-state">{t('index.loadingSessions')}</div>
  {:else if error}
    <div class="sidebar-session-state sidebar-session-state--error">{error}</div>
  {:else if !cwd}
    <div class="sidebar-session-state">{t('session.projectUnavailable')}</div>
  {:else if filteredSessions.length === 0}
    <div class="sidebar-session-state">
      {query.trim() ? t('session.noMatchingProjectSessions') : t('session.noProjectSessions')}
    </div>
  {:else}
    {#each groupedSessions as group (group.bucket)}
      <section class="sidebar-session-group">
        {#if !query.trim()}
          <h2 class="sidebar-session-group-heading">
            <span>{t(dateBucketKeys[group.bucket])}</span>
            <span class="sidebar-session-group-count">{group.sessions.length}</span>
          </h2>
        {/if}
        {#each group.sessions as session (session.id)}
          {@const href = `/session?id=${encodeURIComponent(session.id)}`}
          {@const activeSession = session.id === currentSessionId}
          <a
            class="sidebar-session-row"
            class:sidebar-session-row--active={activeSession}
            class:sidebar-session-row--running={isRunning(session.id)}
            {href}
            aria-current={activeSession ? 'page' : undefined}
            onclick={(event) => onSessionClick(event, href)}
            onpointerenter={() => startPrefetch(session.id)}
            onmousedown={() => startPrefetch(session.id)}
            ontouchstart={() => startPrefetch(session.id)}
          >
            <span class="sidebar-session-indicator" aria-hidden="true"></span>
            <span class="sidebar-session-copy">
              <span class="sidebar-session-heading">
                <span class="sidebar-session-title">{session.name || session.id}</span>
                {#if isRunning(session.id)}
                  <span
                    class="sidebar-running-spinner"
                    data-running-spinner
                    aria-label={t('index.active')}
                    style={spinnerStyle}>{spinnerChar}</span
                  >
                {/if}
              </span>
              <span class="sidebar-session-meta">
                <span title={session.lastActivity}
                  >{formatRelativeTime(session.lastActivity, now)}</span
                >
                {#if sessionModelLabel(session)}
                  <span class="sidebar-session-model" title={sessionModelLabel(session)}
                    >{sessionModelLabel(session)}</span
                  >
                {/if}
              </span>
            </span>
          </a>
        {/each}
      </section>
    {/each}
  {/if}
</div>
