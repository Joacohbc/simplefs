import { $, translate } from './dom.js';

const DEFAULT_TOAST_DURATION_MS = 4500;
const ERROR_TOAST_DURATION_FACTOR = 2;
const MAX_VISIBLE_TOASTS = 4;

export const TOAST_LEVEL = { success: 'success', error: 'error', info: 'info' };

function buildToast(level, message) {
  const toast = $('#toast-template').content.firstElementChild.cloneNode(true);
  const iconTemplate = $(`#toast-icon-${level}`) || $('#toast-icon-info');
  $('.toast-icon', toast).append(iconTemplate.content.cloneNode(true));
  $('.toast-message', toast).textContent = message;
  if (level === TOAST_LEVEL.error) toast.setAttribute('role', 'alert');
  $('.toast-close', toast).addEventListener('click', () => toast.remove());
  return toast;
}

export function showToast(level, message, { durationMs = DEFAULT_TOAST_DURATION_MS } = {}) {
  const region = $('#toast-region');
  if (!region || !$('#toast-template') || !message) return null;
  const toast = buildToast(level, message);
  region.append(toast);
  while (region.children.length > MAX_VISIBLE_TOASTS) region.firstElementChild.remove();
  const isPersistent = durationMs === 0;
  if (!isPersistent) {
    const visibleForMs = level === TOAST_LEVEL.error ? durationMs * ERROR_TOAST_DURATION_FACTOR : durationMs;
    setTimeout(() => toast.remove(), visibleForMs);
  }
  return toast;
}

export async function copyText(value) {
  try {
    await navigator.clipboard.writeText(value);
    showToast(TOAST_LEVEL.success, translate('copied'));
    return true;
  } catch {
    showToast(TOAST_LEVEL.error, translate('copyFailed'));
    return false;
  }
}
