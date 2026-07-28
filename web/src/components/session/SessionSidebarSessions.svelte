<script>
  import { onMount } from 'svelte';
  import { icon, FolderGit2 } from '../../shared/icons.js';
  import { t } from '../../shared/i18n.js';
  import { handleNavClick } from '../../shared/navigation.js';
  import {
    defaultFetchSessions,
    formatRelativeTime,
    normalizeSession,
    sessionModelLabel,
    sessionSearchText,
  } from '../../index/sessions.js';
  import { prefetchSession } from '../../routes/session-prefetch.js';
  import { sessionRuntime } from '../../session/session-runtime.js';

  let { cwd = '', currentSessionId = '', fetchSessions = defaultFetchSessions } = $props();

  let sessions = $state([]);
  let query = $state('');
  let loading = $state(true);
  let error = $state('');
  let now = $state(Date.now());

  const projectName = $derived(cwd.split(/[\\/]/).filter(Boolean).at(-1) || cwd);
  const filteredSessions = $derived.by(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) return sessions;
    return sessions.filter((session) =>
      sessionSearchText(session).toLowerCase().includes(normalizedQuery),
    );
  });

  function startPrefetch(sessionId) {
    if (sessionId) prefetchSession(sessionId);
  }

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
  <input
    type="search"
    class="sidebar-search"
    bind:value={query}
    placeholder={t('session.searchProjectSessions')}
    aria-label={t('session.searchProjectSessions')}
  />
  {#if cwd}
    <div class="sidebar-project" title={cwd}>
      <span class="sidebar-project-icon">{@html icon(FolderGit2, { size: 13 })}</span>
      <span class="sidebar-project-name">{projectName}</span>
      {#if !loading && !error}
        <span class="sidebar-project-count">{sessions.length}</span>
      {/if}
    </div>
  {/if}
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
    {#each filteredSessions as session (session.id)}
      {@const href = `/session?id=${encodeURIComponent(session.id)}`}
      {@const activeSession = session.id === currentSessionId}
      <a
        class="sidebar-session-row"
        class:sidebar-session-row--active={activeSession}
        {href}
        aria-current={activeSession ? 'page' : undefined}
        onclick={(event) => onSessionClick(event, href)}
        onpointerenter={() => startPrefetch(session.id)}
        onmousedown={() => startPrefetch(session.id)}
        ontouchstart={() => startPrefetch(session.id)}
      >
        <span class="sidebar-session-indicator" aria-hidden="true"></span>
        <span class="sidebar-session-copy">
          <span class="sidebar-session-title">{session.name || session.id}</span>
          <span class="sidebar-session-meta">
            <span title={session.lastActivity}>{formatRelativeTime(session.lastActivity, now)}</span
            >
            {#if sessionModelLabel(session)}
              <span class="sidebar-session-model">{sessionModelLabel(session)}</span>
            {/if}
          </span>
        </span>
      </a>
    {/each}
  {/if}
</div>
