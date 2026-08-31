<script>
  // Project detail panel — opens as a FullScreenSheet from the index page.
  // Shows project metadata, git status, open issues, and open PRs.
  import { t } from '../../shared/i18n.js';
  import FullScreenSheet from '../session/FullScreenSheet.svelte';
  import {
    icon,
    ExternalLink,
    FolderOpen,
    Pencil,
    FolderGit2,
    ListTree,
    GitFork,
    CircleHelp,
    CircleAlert,
    FileText,
    Loader,
  } from '../../shared/icons.js';
  import { fetchProjectData, updateProjectName } from '../../project/project-page-data.js';

  let { open = $bindable(false), projectPath = '' } = $props();

  let loading = $state(false);
  let error = $state('');
  let project = $state(null);
  let gitInfo = $state(null);
  let openIssues = $state([]);
  let openPRs = $state([]);
  let sessionCount = $state(0);
  let editingName = $state(false);
  let editName = $state('');

  $effect(() => {
    if (open && projectPath && !project) {
      loadData();
    } else if (!open) {
      project = null;
      gitInfo = null;
      openIssues = [];
      openPRs = [];
      sessionCount = 0;
      error = '';
      editingName = false;
    }
  });

  async function loadData() {
    loading = true;
    error = '';
    try {
      const data = await fetchProjectData(projectPath);
      project = data.project;
      gitInfo = data.gitInfo;
      openIssues = data.openIssues || [];
      openPRs = data.openPRs || [];
      sessionCount = data.sessionCount || 0;
      editName = data.project?.name || '';
    } catch (e) {
      error = e.message || t('project.loadFailed');
    } finally {
      loading = false;
    }
  }

  async function saveName() {
    const name = editName.trim();
    if (!name) return;
    try {
      await updateProjectName(projectPath, name);
      project = { ...project, name };
      editingName = false;
    } catch (e) {
      error = t('project.nameSaveFailed');
    }
  }

  function formatSessionCount(n) {
    return n === 1 ? t('index.sessionCountOne') : t('project.sessionCount', { count: n });
  }
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG -->
<FullScreenSheet
  bind:open
  title={t('project.pageTitle')}
  backdropClass="project-sheet-backdrop"
  panelClass="project-sheet-panel"
  bodyClass="project-sheet-body"
>
  <div class="project-palette">
    {#if loading}
      <div class="project-loading">
        {@html icon(Loader, { size: 20 })}
        <span>{t('project.loading')}</span>
      </div>
    {:else if error && !project}
      <div class="project-empty">
        {@html icon(CircleAlert, { size: 20 })}
        <span>{error}</span>
      </div>
    {:else if project}
      <div class="project-content">
        <!-- Project Identity Header -->
        <div class="project-head">
          {#if editingName}
            <div class="project-name-edit">
              <input
                type="text"
                bind:value={editName}
                class="project-name-input"
                onkeydown={(e) => {
                  if (e.key === 'Enter') saveName();
                  if (e.key === 'Escape') editingName = false;
                }}
              />
              <button class="project-btn" onclick={saveName}>{t('project.saveName')}</button>
              <button class="project-btn-secondary" onclick={() => (editingName = false)}
                >{t('project.cancelEdit')}</button
              >
            </div>
          {:else}
            <div class="project-name-row">
              <div class="project-name" title={project.projectPath}>
                {project.name}
              </div>
              <button
                class="project-edit-btn"
                type="button"
                aria-label={t('project.editName')}
                onclick={() => {
                  editName = project?.name || '';
                  editingName = true;
                }}>{@html icon(Pencil, { size: 14 })}</button
              >
            </div>
          {/if}
          <div class="project-meta-row">
            {#if project.repo}
              <a
                class="project-repo-link"
                href={'https://github.com/' + project.repo}
                target="_blank"
                rel="noopener noreferrer"
              >
                {@html icon(ExternalLink, { size: 13 })}
                <span>{project.repo}</span>
              </a>
            {/if}
            <span class="project-meta-sep">·</span>
            <span class="project-meta-sessions">
              {@html icon(FolderOpen, { size: 13 })}
              {formatSessionCount(sessionCount)}
            </span>
          </div>
          <div class="project-path-row">
            {@html icon(FolderOpen, { size: 12 })}
            <span>{project.path}</span>
          </div>
        </div>

        {#if project.readmeDescription}
          <div class="project-divider"></div>
          <div class="project-group">
            <div class="project-group-header">
              {@html icon(FileText, { size: 13 })}
              <span>{t('project.description')}</span>
            </div>
            <div class="project-description-card">{project.readmeDescription}</div>
          </div>
        {/if}

        {#if gitInfo?.isRepo}
          <div class="project-divider"></div>
          <div class="project-group">
            <div class="project-group-header">
              {@html icon(FolderGit2, { size: 13 })}
              <span>{t('project.gitStatus')}</span>
            </div>
            <div class="project-git-row">
              <span class="project-git-branch">
                {@html icon(ListTree, { size: 13 })}
                {gitInfo.branch}
              </span>
              <div class="project-git-badges">
                {#if gitInfo.dirty}
                  <span class="project-badge is-dirty">{t('project.gitDirty')}</span>
                {:else}
                  <span class="project-badge is-clean">{t('project.gitClean')}</span>
                {/if}
                {#if gitInfo.ahead > 0}
                  <span class="project-badge is-ahead"
                    >{t('project.gitAhead', { n: gitInfo.ahead })}</span
                  >
                {/if}
                {#if gitInfo.behind > 0}
                  <span class="project-badge is-behind"
                    >{t('project.gitBehind', { n: gitInfo.behind })}</span
                  >
                {/if}
              </div>
            </div>
          </div>
        {/if}

        <div class="project-divider"></div>
        <div class="project-group">
          <div class="project-group-header">
            {@html icon(CircleHelp, { size: 13 })}
            <span>{t('project.openIssues')}</span>
            <span class="project-group-count">{openIssues.length}</span>
          </div>
          {#if openIssues.length === 0}
            <div class="project-empty-state">
              {@html icon(CircleHelp, { size: 18 })}
              <span>{t('project.noIssues')}</span>
            </div>
          {:else}
            <div class="project-list">
              {#each openIssues as issue (issue.number)}
                <a class="project-item" href={issue.url} target="_blank" rel="noopener noreferrer">
                  <span class="project-item-number">#{issue.number}</span>
                  <span class="project-item-title">{issue.title}</span>
                </a>
              {/each}
            </div>
          {/if}
        </div>

        <div class="project-divider"></div>
        <div class="project-group">
          <div class="project-group-header">
            {@html icon(GitFork, { size: 13 })}
            <span>{t('project.openPRs')}</span>
            <span class="project-group-count">{openPRs.length}</span>
          </div>
          {#if openPRs.length === 0}
            <div class="project-empty-state">
              {@html icon(GitFork, { size: 18 })}
              <span>{t('project.noPRs')}</span>
            </div>
          {:else}
            <div class="project-list">
              {#each openPRs as pr (pr.number)}
                <a class="project-item" href={pr.url} target="_blank" rel="noopener noreferrer">
                  <span class="project-item-number">#{pr.number}</span>
                  <span class="project-item-title">{pr.title}</span>
                </a>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</FullScreenSheet>
