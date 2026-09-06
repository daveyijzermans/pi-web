// Shared reactive archived flag for the open session. SessionPage seeds it from
// the /api/session payload; CommandMenu toggles it via /api/archive-session.
export const sessionArchived = $state({ value: false });

export function setSessionArchived(value) {
  sessionArchived.value = !!value;
}
