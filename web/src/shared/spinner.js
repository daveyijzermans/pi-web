// Shared running-indicator spinner: config (runcat or braille frames, per the
// user's spinner-style setting) and the frame-cycling animation loop that was
// previously copy-pasted into every component showing a spinner.

export function getSpinnerConfig(windowImpl = typeof window !== 'undefined' ? window : null) {
  let style = 'runcat';
  try {
    if (windowImpl && windowImpl.localStorage) {
      const saved = windowImpl.localStorage.getItem('pi-sessions:spinner-style');
      if (saved === 'braille') {
        style = 'braille';
      }
    }
  } catch (_) {}

  if (style === 'braille') {
    return {
      frames: ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'],
      fontFamily: 'monospace',
      interval: 80,
      width: '12px',
    };
  } else {
    // runcat frames mapping to unicode private use area characters in runcat.ttf font
    return {
      frames: ['', '', '', '', ''],
      fontFamily: "'runcat', monospace",
      interval: 100,
      width: '18px',
    };
  }
}

/**
 * Cycle the spinner frames, calling onFrame(char, config) for each — the first
 * frame fires synchronously so callers can also derive their style string from
 * the config once. Returns a dispose function that stops the cycle.
 */
export function animateSpinner(
  onFrame,
  { windowImpl = typeof window !== 'undefined' ? window : null } = {},
) {
  const config = getSpinnerConfig(windowImpl);
  let frame = 0;
  onFrame(config.frames[0] || '', config);
  const setIntervalImpl = windowImpl?.setInterval?.bind(windowImpl) || setInterval;
  const clearIntervalImpl = windowImpl?.clearInterval?.bind(windowImpl) || clearInterval;
  const timer = setIntervalImpl(() => {
    frame = (frame + 1) % config.frames.length;
    onFrame(config.frames[frame] || '', config);
  }, config.interval || 100);
  return () => clearIntervalImpl(timer);
}

/** The inline style string every spinner element derives from its config. */
export function spinnerStyleFor(config) {
  return `font-family:${config.fontFamily};width:${config.width}`;
}
