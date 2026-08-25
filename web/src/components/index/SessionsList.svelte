<script>
  import { onMount } from 'svelte';
  import { icon, ChevronDown, Info, Archive } from '../../shared/icons.js';
  import { t } from '../../shared/i18n.js';
  import {
    activityMs,
    collapsedProjectsStorageKey,
    filterSessions,
    groupSessionsByProject,
    sessionsCountLabel,
  } from '../../index/sessions.js';
  import SessionCard from './SessionCard.svelte';
  import { getGitInfoByPath } from '../../session/chat/git-api.js';

  let {
    sessions = [],
    layout = 'timeline',
    query = '',
    projects = [],
    runningSessionIds = new Set(),
    runningStatuses = new Map(),
    loading = false,
    layoutReady = false,
    onArchive = null,
    onArchiveProject = null,
    onNewSession = null,
    onViewProject = null,
    archivedProjects = new Set(),
  } = $props();

  let now = $state(Date.now());
  let collapsed = $state({});
  let archivedOpen = $state({});
  let timelineArchivedOpen = $state(false);
  let archivedProjectsOpen = $state(false);
  let gitInfos = $state({});

  const visibleSessions = $derived(filterSessions(sessions, query));
  const isTimeline = $derived(layout === 'timeline');
  const searching = $derived(String(query || '').trim() !== '');
  const projectNames = $derived(Object.fromEntries(projects.map((p) => [p.path, p.name || ''])));

  const allGroups = $derived(isTimeline ? [] : groupSessionsByProject(visibleSessions));
  const groups = $derived(
    searching ? allGroups : allGroups.filter((g) => !archivedProjects.has(g.project)),
  );
  const archivedGroups = $derived(
    searching ? [] : allGroups.filter((g) => archivedProjects.has(g.project)),
  );

  const timelineSorted = $derived(
    [...visibleSessions].sort((a, b) => activityMs(b) - activityMs(a)),
  );
  const timelineActive = $derived(
    searching ? timelineSorted : timelineSorted.filter((session) => !session.archived),
  );
  const timelineArchived = $derived(
    searching ? [] : timelineSorted.filter((session) => session.archived),
  );

  function toggleTimelineArchived() {
    timelineArchivedOpen = !timelineArchivedOpen;
  }

  function splitArchived(sessionList) {
    const active = [];
    const archived = [];
    for (const session of sessionList) {
      if (session.archived) archived.push(session);
      else active.push(session);
    }
    return { active, archived };
  }

  function readCollapsed() {
    try {
      const raw = localStorage.getItem(collapsedProjectsStorageKey);
      if (!raw) return {};
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch {
      return {};
    }
  }

  function writeCollapsed(state) {
    try {
      localStorage.setItem(collapsedProjectsStorageKey, JSON.stringify(state));
    } catch {}
  }

  function toggleProject(project) {
    collapsed = { ...collapsed, [project]: collapsed[project] ? undefined : 1 };
    if (!collapsed[project]) {
      const next = { ...collapsed };
      delete next[project];
      collapsed = next;
    }
    writeCollapsed(collapsed);
  }

  function toggleArchived(project) {
    archivedOpen = { ...archivedOpen, [project]: !archivedOpen[project] };
  }

  function runningCountFor(group) {
    return group.sessions.filter((session) => runningSessionIds.has(session.id)).length;
  }

  function fetchGitInfoForProject(project) {
    if (gitInfos[project] !== undefined) return;
    getGitInfoByPath(project).then(
      (info) => {
        gitInfos = { ...gitInfos, [project]: info };
      },
      () => {
        gitInfos = { ...gitInfos, [project]: null };
      },
    );
  }

  $effect(() => {
    for (const group of groups) {
      fetchGitInfoForProject(group.project);
    }
  });

  onMount(() => {
    collapsed = readCollapsed();
    const timer = setInterval(() => {
      now = Date.now();
    }, 60000);
    return () => clearInterval(timer);
  });
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG and rendered session markdown -->

<div
  class="content"
  class:content--timeline={isTimeline}
  class:index-layout-ready={layoutReady}
  data-sessions-content
>
  {#if loading && sessions.length === 0}
    <div class="empty-state">
      <h3>{t('index.loadingSessions')}</h3>
      <p>{t('index.loadingSessionsHint')}</p>
    </div>
  {:else if sessions.length === 0}
    <div class="empty-state">
      <h3>{t('index.noSessionsYet')}</h3>
      <p>{t('index.noSessionsYetHint')}</p>
    </div>
  {:else if visibleSessions.length === 0}
    <div class="empty-state">
      <h3>{t('index.noSessions')}</h3>
      <p>{t('index.noSessionsHint')}</p>
    </div>
  {:else if isTimeline}
    <div class="session-grid session-grid--timeline session-grid--flat">
      {#each timelineActive as session (session.id)}
        <SessionCard
          {session}
          running={runningSessionIds.has(session.id)}
          runningStatus={runningStatuses.get(session.id)}
          {now}
          {onArchive}
          showProject
        />
      {/each}
    </div>
    {#if timelineArchived.length > 0}
      <button
        class="archived-toggle"
        type="button"
        aria-expanded={String(timelineArchivedOpen)}
        onclick={toggleTimelineArchived}
      >
        <span class="project-chevron" aria-hidden="true"
          >{@html icon(ChevronDown, { size: 12 })}</span
        >
        {t('index.archivedCount', { count: timelineArchived.length })}
      </button>
      {#if timelineArchivedOpen}
        <div class="session-grid session-grid--timeline session-grid--flat archived-grid">
          {#each timelineArchived as session (session.id)}
            <SessionCard
              {session}
              running={runningSessionIds.has(session.id)}
              runningStatus={runningStatuses.get(session.id)}
              {now}
              {onArchive}
              showProject
            />
          {/each}
        </div>
      {/if}
    {/if}
  {:else}
    {#each groups as group (group.project + ':' + group.sessions[0]?.id)}
      {@const runningCount = runningCountFor(group)}
      {@const isCollapsed = !!collapsed[group.project]}
      {@const split = splitArchived(group.sessions)}
      {@const cards = searching ? group.sessions : split.active}
      {@const archOpen = !!archivedOpen[group.project]}
      <div class="project-group" class:collapsed={isCollapsed} data-project={group.project}>
        <div class="project-header">
          <button
            class="project-toggle"
            type="button"
            aria-expanded={String(!isCollapsed)}
            onclick={() => toggleProject(group.project)}
          >
            <span class="project-chevron" aria-hidden="true"
              >{@html icon(ChevronDown, { size: 12 })}</span
            >
            <span class="project-name-line">
              <span class="project-name">{projectNames[group.project] || group.project}</span>
              {#if gitInfos[group.project]?.isRepo}
                {@const gitInfo = gitInfos[group.project]}
                <span class="pi-git-status">
                  {#if gitInfo.modified > 0}<span class="pi-git-status-badge pi-git-status-modified"
                      >M {gitInfo.modified}</span
                    >{/if}
                  {#if gitInfo.added > 0}<span class="pi-git-status-badge pi-git-status-added"
                      >N {gitInfo.added}</span
                    >{/if}
                  {#if gitInfo.deleted > 0}<span class="pi-git-status-badge pi-git-status-deleted"
                      >D {gitInfo.deleted}</span
                    >{/if}
                  {#if gitInfo.ahead > 0}<span class="pi-git-status-badge pi-git-status-ahead"
                      >↑{gitInfo.ahead}</span
                    >{/if}
                  {#if gitInfo.behind > 0}<span class="pi-git-status-badge pi-git-status-behind"
                      >↓{gitInfo.behind}</span
                    >{/if}
                </span>
              {/if}
            </span>
            <span
              class="project-count"
              data-project-count
              data-running={runningCount}
              data-total={cards.length}
            >
              {runningCount > 0
                ? t('index.activeCount', { count: runningCount })
                : sessionsCountLabel(cards.length)}
            </span>
          </button>
          {#if onViewProject}
            <button
              type="button"
              class="project-view-btn"
              aria-label={t('index.viewProject')}
              title={t('index.viewProject')}
              onclick={() => onViewProject(group.project)}>{@html icon(Info, { size: 14 })}</button
            >
          {/if}
          {#if onArchiveProject}
            <button
              type="button"
              class="project-archive-btn"
              aria-label={t('index.archiveProject')}
              title={t('index.archiveProject')}
              onclick={() => onArchiveProject(group.project, true)}
            >
              {@html icon(Archive, { size: 14 })}
            </button>
          {/if}
          <button
            class="project-new-btn"
            type="button"
            aria-label={t('index.newSessionInProject')}
            title={t('index.newSessionInProject')}
            onclick={() => onNewSession && onNewSession(group.project)}
          >
            +
          </button>
        </div>
        <div class="session-grid">
          {#each cards as session (session.id)}
            <SessionCard
              {session}
              running={runningSessionIds.has(session.id)}
              runningStatus={runningStatuses.get(session.id)}
              {now}
              {onArchive}
            />
          {/each}
        </div>
        {#if !searching && split.archived.length > 0}
          <button
            class="archived-toggle"
            type="button"
            aria-expanded={String(archOpen)}
            onclick={() => toggleArchived(group.project)}
          >
            <span class="project-chevron" aria-hidden="true"
              >{@html icon(ChevronDown, { size: 12 })}</span
            >
            {t('index.archivedCount', { count: split.archived.length })}
          </button>
          {#if archOpen}
            <div class="session-grid archived-grid">
              {#each split.archived as session (session.id)}
                <SessionCard
                  {session}
                  running={runningSessionIds.has(session.id)}
                  runningStatus={runningStatuses.get(session.id)}
                  {now}
                  {onArchive}
                />
              {/each}
            </div>
          {/if}
        {/if}
      </div>
    {/each}
    {#if archivedGroups.length > 0}
      <button
        class="archived-toggle"
        type="button"
        aria-expanded={String(archivedProjectsOpen)}
        onclick={() => (archivedProjectsOpen = !archivedProjectsOpen)}
      >
        <span class="project-chevron" aria-hidden="true"
          >{@html icon(ChevronDown, { size: 12 })}</span
        >
        {t('index.archivedProjectsCount', { count: archivedGroups.length })}
      </button>
      {#if archivedProjectsOpen}
        {#each archivedGroups as group (group.project + ':' + group.sessions[0]?.id)}
          {@const runningCount = runningCountFor(group)}
          {@const isCollapsed = !!collapsed[group.project]}
          {@const split = splitArchived(group.sessions)}
          {@const cards = searching ? group.sessions : split.active}
          <div class="project-group" class:collapsed={isCollapsed} data-project={group.project}>
            <div class="project-header">
              <button
                class="project-toggle"
                type="button"
                aria-expanded={String(!isCollapsed)}
                onclick={() => toggleProject(group.project)}
              >
                <span class="project-chevron" aria-hidden="true"
                  >{@html icon(ChevronDown, { size: 12 })}</span
                >
                <span class="project-name-line">
                  <span class="project-name">{projectNames[group.project] || group.project}</span>
                </span>
                <span
                  class="project-count"
                  data-project-count
                  data-running={runningCount}
                  data-total={cards.length}
                >
                  {runningCount > 0
                    ? t('index.activeCount', { count: runningCount })
                    : sessionsCountLabel(cards.length)}
                </span>
              </button>
              {#if onArchiveProject}
                <button
                  type="button"
                  class="project-archive-btn"
                  aria-label={t('index.unarchiveProject')}
                  title={t('index.unarchiveProject')}
                  onclick={() => onArchiveProject(group.project, false)}
                >
                  {@html icon(Archive, { size: 14 })}
                </button>
              {/if}
            </div>
            <div class="session-grid">
              {#each cards as session (session.id)}
                <SessionCard
                  {session}
                  running={runningSessionIds.has(session.id)}
                  runningStatus={runningStatuses.get(session.id)}
                  {now}
                  {onArchive}
                />
              {/each}
            </div>
          </div>
        {/each}
      {/if}
    {/if}
  {/if}
</div>
