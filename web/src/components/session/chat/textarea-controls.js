import { matchesAction } from '../../../shared/keybindings.js';

export function setupTextareaControls({
  windowImpl = window,
  textarea,
  shell,
  form,
  collapseInputButton = null,
  isMobileTextInputMode = () => false,
  getSlashSelector = () => null,
  getMentionSelector = () => null,
  getThinkingSelector = () => null,
  getModelSelector = () => null,
  updateSendEnabled = () => {},
  updateComposerHeight = () => {},
} = {}) {
  function autoResize() {
    if (!textarea || (shell && shell.classList.contains('expanded'))) return;
    textarea.style.height = 'auto';
    const cs = windowImpl.getComputedStyle(textarea);
    const max = parseFloat(cs.maxHeight) || 200;
    const min = parseFloat(cs.minHeight) || 36;
    const lineHeight = parseFloat(cs.lineHeight) || 18;
    const padding = (parseFloat(cs.paddingTop) || 0) + (parseFloat(cs.paddingBottom) || 0);
    const next = Math.max(min, Math.min(textarea.scrollHeight, max));
    textarea.style.height = next + 'px';
    shell?.classList.toggle('input-multiline', textarea.scrollHeight > lineHeight + padding + 2);
    updateComposerHeight();
  }

  const onInput = () => {
    shell?.classList.remove('input-collapsed');
    autoResize();
    updateSendEnabled();
  };

  const onFocus = () => {
    if (!shell?.classList.contains('input-collapsed')) return;
    shell.classList.remove('input-collapsed');
    autoResize();
  };

  const onBlur = () => {
    if (textarea && textarea.value.trim() === '') {
      shell?.classList.remove('input-collapsed');
      autoResize();
    }
  };

  const onCollapseClick = () => {
    shell?.classList.add('input-collapsed');
    updateComposerHeight();
  };

  const onKeydown = (event) => {
    if (getSlashSelector()?.handleKeydown?.(event)) return;
    if (getMentionSelector()?.handleKeydown?.(event)) return;
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      if (isMobileTextInputMode()) return;
      event.preventDefault();
      form?.requestSubmit?.();
    }
    if (matchesAction('cycle-thinking-level', event)) {
      event.preventDefault();
      getThinkingSelector()?.cycle?.();
    }
    if (matchesAction('open-model-selector', event)) {
      event.preventDefault();
      getModelSelector()?.open?.();
    }
  };

  if (textarea) {
    textarea.addEventListener('input', onInput);
    textarea.addEventListener('keydown', onKeydown);
    textarea.addEventListener('focus', onFocus);
    textarea.addEventListener('blur', onBlur);
    autoResize();
  }
  collapseInputButton?.addEventListener('click', onCollapseClick);
  updateSendEnabled();

  return {
    autoResize,
    dispose: () => {
      textarea?.removeEventListener('input', onInput);
      textarea?.removeEventListener('keydown', onKeydown);
      textarea?.removeEventListener('focus', onFocus);
      textarea?.removeEventListener('blur', onBlur);
      collapseInputButton?.removeEventListener('click', onCollapseClick);
    },
  };
}
