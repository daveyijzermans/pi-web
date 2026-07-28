<script>
  import { icon, Clock, ListTree, PanelLeftClose, X } from '../../shared/icons.js';
  import { t } from '../../shared/i18n.js';
  import { getSessionModel } from '../../session/session-context.js';
  import { sessionRuntime } from '../../session/session-runtime.js';
  import { getSessionRuntime } from '../../session/session-runtime-context.js';
  import SessionSidebarSessions from './SessionSidebarSessions.svelte';
  import SessionTreeNodes from './SessionTreeNodes.svelte';

  let { cwd = '', sessionId = '' } = $props();

  const model = getSessionModel();
  let activeTab = $state('sessions');

  // Route a tree-node click through the shared navigator so message content
  // scrolls after the reactive render. Navigate to the newest leaf under the
  // clicked node, with the clicked node as the scroll target; auto-close the
  // drawer on mobile.
  function onNavigate(id) {
    const leaf = model?.newestLeaf(id) || id;
    const navigateTo = getSessionRuntime().navigateTo;
    navigateTo?.(leaf, 'target', id);
    if (sessionRuntime.layout?.isMobileLayout?.()) sessionRuntime.layout?.closeSidebar?.();
  }
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG and rendered session markdown -->

<aside id="sidebar">
  <div class="sidebar-tabbar" role="tablist" aria-label={t('session.sidebarTabs')}>
    <button
      type="button"
      class="sidebar-tab"
      class:active={activeTab === 'sessions'}
      role="tab"
      aria-selected={activeTab === 'sessions'}
      aria-controls="sidebar-sessions-panel"
      onclick={() => (activeTab = 'sessions')}
    >
      {@html icon(Clock, { size: 13 })}<span>{t('session.sessionsTab')}</span>
    </button>
    <button
      type="button"
      class="sidebar-tab"
      class:active={activeTab === 'outline'}
      role="tab"
      aria-selected={activeTab === 'outline'}
      aria-controls="sidebar-outline-panel"
      onclick={() => (activeTab = 'outline')}
    >
      {@html icon(ListTree, { size: 13 })}<span>{t('session.outlineTab')}</span>
    </button>
    <span class="sidebar-tabbar-spacer"></span>
    <button id="hide-sidebar" class="hide-sidebar" title={t('session.hideSidebar')}
      >{@html icon(PanelLeftClose, { size: 14 })}</button
    ><button
      id="sidebar-close"
      class="sidebar-close"
      title={t('common.close')}
      aria-label={t('session.closeSidebar')}>{@html icon(X, { size: 14 })}</button
    >
  </div>

  <div
    id="sidebar-sessions-panel"
    class="sidebar-panel"
    role="tabpanel"
    aria-label={t('session.sessionsTab')}
    hidden={activeTab !== 'sessions'}
  >
    <SessionSidebarSessions {cwd} currentSessionId={sessionId} />
  </div>

  <div
    id="sidebar-outline-panel"
    class="sidebar-panel"
    role="tabpanel"
    aria-label={t('session.outlineTab')}
    hidden={activeTab !== 'outline'}
  >
    <div class="sidebar-header">
      <div class="sidebar-controls">
        <input
          type="search"
          class="sidebar-search"
          id="tree-search"
          placeholder={t('session.searchOutline')}
          aria-label={t('session.searchOutline')}
        />
      </div>
      <div class="sidebar-filters">
        <button
          class="filter-btn active"
          data-filter="default"
          title={t('session.filterDefaultTitle')}>{t('session.filterDefault')}</button
        ><button class="filter-btn" data-filter="no-tools" title={t('session.filterNoToolsTitle')}
          >{t('session.filterNoTools')}</button
        ><button class="filter-btn" data-filter="user-only" title={t('session.filterUserTitle')}
          >{t('session.filterUser')}</button
        ><button
          class="filter-btn"
          data-filter="labeled-only"
          title={t('session.filterLabeledTitle')}>{t('session.filterLabeled')}</button
        ><button class="filter-btn" data-filter="all" title={t('session.filterAllTitle')}
          >{t('session.filterAll')}</button
        >
      </div>
    </div>
    {#if model}<SessionTreeNodes {model} {onNavigate} />{:else}<div
        class="tree-container"
        id="tree-container"
      ></div>
      <div class="tree-status" id="tree-status"></div>{/if}
  </div>
</aside>
<div
  id="sidebar-resizer"
  role="separator"
  aria-orientation="vertical"
  aria-label={t('session.resizeTree')}
></div>
