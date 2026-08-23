import { hasTextContent } from './session-filter.js';

const REPARENTABLE_ORPHAN_TYPES = new Set(['model_change', 'thinking_level_change']);

// Session-metadata entries (a model change, a thinking-level change) are sometimes
// written with a null parentId mid-session — e.g. after an unarchive. Left as-is
// each one becomes a second tree root, so the conversation that continues from it
// renders as a separate branch and the earlier history drops off the active path
// (issue #123). Re-thread every such orphan onto the entry that precedes it so the
// session stays a single spine. Only non-message metadata is touched: real message
// branches carry valid parentIds and a genuine root/branch message is left alone.
export function relinkOrphanMetadata(entries = []) {
  let lastId = null;
  return entries.map((entry) => {
    if (!entry?.id) return entry;
    const orphaned = entry.parentId == null || entry.parentId === entry.id;
    const relinked =
      lastId && orphaned && REPARENTABLE_ORPHAN_TYPES.has(entry.type)
        ? { ...entry, parentId: lastId }
        : entry;
    lastId = entry.id;
    return relinked;
  });
}

export function buildTree(entries = [], labelMap = new Map()) {
  const nodeMap = new Map();
  const roots = [];

  // Deduplicate by ID keeping the last occurrence (consistent with byId Map)
  const seenIds = new Set();
  const treeEntries = [];
  for (let i = entries.length - 1; i >= 0; i -= 1) {
    const entry = entries[i];
    if (!entry?.id) continue;
    if (seenIds.has(entry.id)) continue;
    seenIds.add(entry.id);
    treeEntries.unshift(entry);
  }

  for (const entry of treeEntries) {
    nodeMap.set(entry.id, { entry, children: [], label: labelMap.get(entry.id) });
  }

  for (const entry of treeEntries) {
    const node = nodeMap.get(entry.id);
    if (entry.parentId === null || entry.parentId === undefined || entry.parentId === entry.id) {
      roots.push(node);
    } else {
      const parent = nodeMap.get(entry.parentId);
      if (parent) parent.children.push(node);
      else roots.push(node);
    }
  }

  function sortChildren(node) {
    node.children.sort(
      (a, b) => new Date(a.entry.timestamp).getTime() - new Date(b.entry.timestamp).getTime(),
    );
    node.children.forEach(sortChildren);
  }
  roots.forEach(sortChildren);
  return roots;
}

// A forked session that pi later resumed (and any plain resume) is written as
// several sequential conversation segments, each beginning with its own
// parentId:null root. getPath/buildTree would treat those roots as disconnected,
// so the content pane would render only the last segment while the earlier ones
// linger in the tree as separate roots. Re-link every conversation root after the
// first onto the previous segment's most recent entry so the whole conversation
// forms one chain. The session-header line ({type:'session'}) is metadata, not a
// conversation root, and is left untouched. Returns the input unchanged when there
// is nothing to stitch (the common single-segment case).
export function stitchOrphanRoots(entries = []) {
  let result = entries;
  let prevLeafId = null;
  let seenRoot = false;
  for (let i = 0; i < entries.length; i += 1) {
    const entry = entries[i];
    if (!entry?.id || entry.type === 'session' || entry.type === 'label') continue;
    const isRoot =
      entry.parentId === null || entry.parentId === undefined || entry.parentId === entry.id;
    if (isRoot && seenRoot && prevLeafId) {
      if (result === entries) result = entries.slice();
      result[i] = { ...entry, parentId: prevLeafId };
    }
    if (isRoot) seenRoot = true;
    prevLeafId = entry.id;
  }
  return result;
}

export function buildActivePathIds(targetId, byId = new Map()) {
  const ids = new Set();
  let current = byId.get(targetId);
  while (current) {
    ids.add(current.id);
    if (!current.parentId || current.parentId === current.id) break;
    current = byId.get(current.parentId);
  }
  return ids;
}

export function getPath(targetId, byId = new Map()) {
  const path = [];
  let current = byId.get(targetId);
  while (current) {
    path.unshift(current);
    if (!current.parentId || current.parentId === current.id) break;
    current = byId.get(current.parentId);
  }
  return path;
}

export function buildTreeNodeMap(roots = []) {
  const treeNodeMap = new Map();
  function mapNodes(node) {
    treeNodeMap.set(node.entry.id, node);
    node.children.forEach(mapNodes);
  }
  roots.forEach(mapNodes);
  return treeNodeMap;
}

export function findNewestLeaf(nodeId, rootsOrNodeMap = []) {
  const treeNodeMap =
    rootsOrNodeMap instanceof Map ? rootsOrNodeMap : buildTreeNodeMap(rootsOrNodeMap);
  const node = treeNodeMap.get(nodeId);
  if (!node) return nodeId;

  function newestNavigable(current) {
    for (let i = current.children.length - 1; i >= 0; i -= 1) {
      const candidate = newestNavigable(current.children[i]);
      if (candidate) return candidate;
    }
    return current.entry.type === 'label' ? null : current.entry.id;
  }

  return newestNavigable(node) || nodeId;
}

export function flattenTree(roots, activePathIds) {
  const result = [];
  const multipleRoots = roots.length > 1;
  const containsActive = new Map();

  function markActive(node) {
    let has = activePathIds.has(node.entry.id);
    for (const child of node.children) {
      if (markActive(child)) has = true;
    }
    containsActive.set(node, has);
    return has;
  }
  roots.forEach(markActive);

  const stack = [];
  const orderedRoots = [...roots].sort(
    (a, b) => Number(containsActive.get(b)) - Number(containsActive.get(a)),
  );
  for (let i = orderedRoots.length - 1; i >= 0; i -= 1) {
    const isLast = i === orderedRoots.length - 1;
    stack.push([
      orderedRoots[i],
      multipleRoots ? 1 : 0,
      multipleRoots,
      multipleRoots,
      isLast,
      [],
      multipleRoots,
    ]);
  }

  while (stack.length > 0) {
    const [node, indent, justBranched, showConnector, isLast, gutters, isVirtualRootChild] =
      stack.pop();
    result.push({
      node,
      indent,
      showConnector,
      isLast,
      gutters,
      isVirtualRootChild,
      multipleRoots,
    });

    const children = node.children;
    const multipleChildren = children.length > 1;
    const orderedChildren = [...children].sort(
      (a, b) => Number(containsActive.get(b)) - Number(containsActive.get(a)),
    );
    let childIndent;
    if (multipleChildren) childIndent = indent + 1;
    else if (justBranched && indent > 0) childIndent = indent + 1;
    else childIndent = indent;

    const connectorDisplayed = showConnector && !isVirtualRootChild;
    const currentDisplayIndent = multipleRoots ? Math.max(0, indent - 1) : indent;
    const connectorPosition = Math.max(0, currentDisplayIndent - 1);
    const childGutters = connectorDisplayed
      ? [...gutters, { position: connectorPosition, show: !isLast }]
      : gutters;

    for (let i = orderedChildren.length - 1; i >= 0; i -= 1) {
      const childIsLast = i === orderedChildren.length - 1;
      stack.push([
        orderedChildren[i],
        childIndent,
        multipleChildren,
        multipleChildren,
        childIsLast,
        childGutters,
        false,
      ]);
    }
  }

  return result;
}

/**
 * Walk a raw parent→child path and merge consecutive internal assistant entries
 * (those with no user-facing text) into the next terminal entry. Tool results
 * between internal entries are absorbed into the group's tool calls.
 *
 * Returns a new array where internal assistant entries are collapsed into the
 * terminal entry that follows them. Non-assistant entries (user, model_change,
 * compaction, etc.) pass through unchanged.
 */
export function getGroupedPath(path) {
  const grouped = [];
  let pendingBlocks = [];
  let pendingIds = [];
  let lastAssistantEntry = null;

  for (let i = 0; i < path.length; i += 1) {
    const entry = path[i];
    const msg = entry.type === 'message' ? entry.message : null;

    // Assistant message
    if (msg?.role === 'assistant') {
      lastAssistantEntry = entry;
      // Terminal = final response to the user: no tool calls AND has real text.
      // Everything else is internal and gets merged forward:
      //  - has tool calls → "keep working" (even if also has transitional text)
      //  - no tool calls AND no text → thinking-only placeholder, merge forward
      const hasToolCalls =
        Array.isArray(msg.content) && msg.content.some((b) => b.type === 'toolCall');
      const isInternal = hasToolCalls || !hasTextContent(msg.content);

      if (isInternal) {
        // Collect thinking + text + toolCalls in document order, carrying source timestamp and sourceId
        if (Array.isArray(msg.content)) {
          for (const block of msg.content) {
            if (block.type === 'thinking' && block.thinking?.trim()) {
              pendingBlocks.push({
                type: 'thinking',
                thinking: block.thinking,
                timestamp: entry.timestamp,
                sourceId: entry.id,
              });
            } else if (block.type === 'text' && block.text?.trim()) {
              pendingBlocks.push({
                type: 'text',
                text: block.text,
                timestamp: entry.timestamp,
                sourceId: entry.id,
              });
            } else if (block.type === 'toolCall') {
              pendingBlocks.push({ ...block, timestamp: entry.timestamp, sourceId: entry.id });
            }
          }
        }
        pendingIds.push(entry.id);
      } else {
        // Terminal assistant — merge collected internal content into it
        const memberIds = [...pendingIds, entry.id];
        if (pendingBlocks.length > 0) {
          const mergedContent = mergeAssistantContent(msg.content, pendingBlocks, entry.id);
          grouped.push({
            ...entry,
            id: memberIds[0],
            message: { ...msg, content: mergedContent },
            memberIds,
          });
          pendingBlocks = [];
          pendingIds = [];
        } else {
          const terminalContent = tagTerminalContent(msg.content, entry.id);
          grouped.push({
            ...entry,
            id: memberIds[0],
            message: { ...msg, content: terminalContent },
            memberIds,
          });
          pendingBlocks = [];
          pendingIds = [];
        }
      }
    } else if (msg?.role === 'toolResult') {
      // Tool result between internal entries — skip (already embedded in toolCall)
      if (pendingBlocks.length > 0) {
        continue;
      }
      grouped.push({ ...entry, memberIds: [entry.id] });
    } else if (msg?.role === 'user') {
      // New human turn — flush any internal blocks that never reached a terminal, then the user msg
      if (pendingBlocks.length > 0) {
        grouped.push(buildOrphanGroupEntry(lastAssistantEntry, pendingBlocks, pendingIds));
      }
      // A user turn always ends the previous assistant turn unconditionally,
      // even when there are no pending blocks (e.g. a block-less aborted entry).
      pendingBlocks = [];
      pendingIds = [];
      grouped.push({ ...entry, memberIds: [entry.id] });
    } else {
      // Any other entry mid-turn (custom hook, model_change, compaction, etc.) must NOT split the
      // group — pass it through and keep pendingBlocks alive for the upcoming terminal assistant.
      grouped.push({ ...entry, memberIds: [entry.id] });
    }
  }

  // Flush remaining internal entries (no terminal found — e.g. session ended mid-tool-use)
  if (pendingBlocks.length > 0) {
    grouped.push(buildOrphanGroupEntry(lastAssistantEntry, pendingBlocks, pendingIds));
  }

  return grouped;
}

/**
 * Merge collected blocks from internal entries into a terminal assistant's
 * content, preserving document order.
 */
function mergeAssistantContent(content, pendingBlocks, terminalEntryId) {
  const merged = [...pendingBlocks];

  if (typeof content === 'string') {
    merged.push({ type: 'text', text: content, sourceId: terminalEntryId });
  } else if (Array.isArray(content)) {
    for (const block of content) {
      merged.push({ ...block, sourceId: terminalEntryId });
    }
  }

  return merged;
}

function tagTerminalContent(content, terminalEntryId) {
  if (typeof content === 'string') {
    return [{ type: 'text', text: content, sourceId: terminalEntryId }];
  }
  if (Array.isArray(content)) {
    return content.map((block) => ({ ...block, sourceId: terminalEntryId }));
  }
  return content;
}

/**
 * Build a synthetic assistant entry when internal entries have no terminal
 * to merge into (e.g. session ended mid-tool-use or non-assistant follows).
 */
function buildOrphanGroupEntry(referenceEntry, pendingBlocks, pendingIds) {
  return {
    id: pendingIds[0] || referenceEntry?.id || 'grouped-orphan',
    type: 'message',
    message: {
      role: 'assistant',
      content: [...pendingBlocks],
    },
    timestamp: referenceEntry?.timestamp || '',
    memberIds: [...pendingIds],
  };
}

export function buildTreePrefix(flatNode) {
  const { indent, showConnector, isLast, gutters, isVirtualRootChild, multipleRoots } = flatNode;
  const displayIndent = multipleRoots ? Math.max(0, indent - 1) : indent;
  const connector = showConnector && !isVirtualRootChild ? (isLast ? '└─ ' : '├─ ') : '';
  const connectorPosition = connector ? displayIndent - 1 : -1;
  const totalChars = displayIndent * 3;
  const prefixChars = [];
  for (let i = 0; i < totalChars; i += 1) {
    const level = Math.floor(i / 3);
    const posInLevel = i % 3;
    const gutter = gutters.find((g) => g.position === level);
    if (gutter) prefixChars.push(posInLevel === 0 ? (gutter.show ? '│' : ' ') : ' ');
    else if (connector && level === connectorPosition)
      prefixChars.push(posInLevel === 0 ? (isLast ? '└' : '├') : posInLevel === 1 ? '─' : ' ');
    else prefixChars.push(' ');
  }
  return prefixChars.join('');
}
