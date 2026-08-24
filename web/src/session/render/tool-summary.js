// Pure helpers for resolving a tool call's result/status from the model.
// No Svelte, no DOM — fully unit-testable.

/**
 * Resolve the tool result message object for a given call id.
 * Returns the `message` object (with `.details`, `.isError`, `.content`) or `null`.
 */
export function resolveToolResult(model, callId) {
  if (!model?.entries) return null;
  for (const entry of model.entries) {
    if (
      entry.type === 'message' &&
      entry.message?.role === 'toolResult' &&
      entry.message.toolCallId === callId
    ) {
      return entry.message;
    }
  }
  return null;
}

/**
 * Resolve the status of a single tool call by scanning the model entries.
 * Returns 'success' | 'error' | 'pending'.
 */
export function resolveToolStatus(model, callId) {
  const result = resolveToolResult(model, callId);
  if (!result) return 'pending';
  return result.isError ? 'error' : 'success';
}
