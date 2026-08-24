export function createComposerSendState({
  textarea = null,
  sendButton = null,
  getAttachments = () => ({ hasAttachments: () => false }),
  // Non-empty means the session is busy in a way that must block a new turn
  // (an in-flight compaction, or a terminal pi process mid-turn). The send
  // button is disabled regardless of content while this is set.
  getBusyReason = () => '',
} = {}) {
  function hasComposerContent() {
    const value = textarea ? textarea.value : '';
    return (value && value.trim().length > 0) || !!getAttachments()?.hasAttachments?.();
  }

  function updateSendEnabled() {
    if (!sendButton) return;
    // Don't fight transient sending/disabled state set by sendChatMessage.
    if (sendButton.dataset.sending === '1') return;
    sendButton.disabled = !!getBusyReason() || !hasComposerContent();
  }

  return {
    hasComposerContent,
    updateSendEnabled,
  };
}
