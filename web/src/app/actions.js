import { $, isTypingTarget } from './dom.js';
import { filterByCategory, scheduleSearch } from './browsing.js';
import { openClipboardModal, pasteFromClipboard, setExtension } from './clipboard.js';
import { isSpeedDialOpen, setSpeedDial, toggleSidebar } from './layout.js';
import { closeModal, openModal, topModal, trapFocus } from './modals.js';
import { copyPreview, resetImage, rotateImage, zoomImageBy } from './preview.js';
import { copyText } from './toasts.js';
import { currentState, navigate, stateFromLocation, syncChrome } from './view-state.js';

const ONE_YEAR_SECONDS = 31536000;
const PRIMARY_MOUSE_BUTTON = 0;

function encodePath(path) {
  return path.split('/').filter(Boolean).map(encodeURIComponent).join('/');
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

function saveThemePreference(isDark) {
  try {
    localStorage.setItem('theme', isDark ? 'dark' : 'light');
    return true;
  } catch {
    return false;
  }
}

function clearSearch() {
  const searchInput = $('#global-search-input');
  if (searchInput) searchInput.value = '';
  navigate({ query: '' });
}

function nextSortOrder(column) {
  const state = currentState();
  const isAscendingOnSameColumn = state.sort === column && state.order === 'asc';
  return isAscendingOnSameColumn ? 'desc' : 'asc';
}

const actions = {
  'toggle-theme'() {
    saveThemePreference(document.documentElement.classList.toggle('dark'));
  },
  'set-lang'(element) {
    const url = new URL(window.location.href);
    url.searchParams.set('lang', element.dataset.lang);
    window.location.href = url.toString();
  },
  'toggle-sidebar'() { toggleSidebar(); },
  'toggle-speed-dial'() { setSpeedDial(!isSpeedDialOpen()); },
  'open-modal'(element) {
    toggleSidebar(false);
    openModal(element.dataset.target);
  },
  'open-folder-modal'() {
    setSpeedDial(false);
    syncChrome();
    openModal('folder-modal');
  },
  'open-clipboard-modal'() { openClipboardModal(); },
  'close-modal'(element) { closeModal(element.closest('[data-modal]') || topModal()); },
  'paste-clipboard'() { pasteFromClipboard(); },
  'set-ext'(element) { setExtension(element.dataset.ext); },
  'trigger-upload'() {
    setSpeedDial(false);
    $('#global-file-picker').click();
  },
  'trigger-zip-upload'() {
    setSpeedDial(false);
    $('#global-zip-picker').click();
  },
  'set-view'(element) {
    const view = element.dataset.view;
    document.cookie = `view=${view}; path=/; max-age=${ONE_YEAR_SECONDS}; SameSite=Lax`;
    navigate({ view });
  },
  'column-sort'(element) {
    const column = element.dataset.sort;
    navigate({ sort: column, order: nextSortOrder(column) });
  },
  'filter'(element) { filterByCategory(element.dataset.category, element); },
  'clear-search'() { clearSearch(); },
  'copy-curl'(element) {
    const fileUrl = `${window.location.origin}/api/v1/files/${encodePath(element.dataset.path)}`;
    copyText(`curl -fL -H "Authorization: Bearer $SIMPLEFS_TOKEN" -o ${shellQuote(element.dataset.name)} ${shellQuote(fileUrl)}`);
  },
  'copy-link'(element) {
    copyText(`${window.location.origin}/download?path=${encodeURIComponent(element.dataset.path)}&download=1`);
  },
  'copy-code'(element) {
    const codeBlock = element.closest('li').querySelector('pre');
    if (codeBlock) copyText(codeBlock.textContent);
  },
  'copy-preview'(element) { copyPreview(element); },
  'print'() { window.print(); },
  'img-zoom'(element) { zoomImageBy(Number(element.dataset.delta)); },
  'img-reset'() { resetImage(); },
  'img-rotate'() { rotateImage(); },
};

function runAction(event) {
  const actionElement = event.target.closest('[data-action]');
  const action = actionElement && actions[actionElement.dataset.action];
  if (!action) return false;
  event.preventDefault();
  action(actionElement, event);
  return true;
}

function isPlainPrimaryClick(event) {
  const hasModifier = event.metaKey || event.ctrlKey || event.shiftKey || event.altKey;
  return event.button === PRIMARY_MOUSE_BUTTON && !hasModifier;
}

function followNavigationLink(event) {
  const link = event.target.closest('a[data-nav]');
  if (!link || !isPlainPrimaryClick(event)) return false;
  event.preventDefault();
  toggleSidebar(false);
  navigate({ ...stateFromLocation(new URL(link.href)), view: currentState().view, query: '' });
  return true;
}

function closeOnOutsideClick(event) {
  const modal = event.target.closest('[data-modal]');
  if (modal && event.target === modal) closeModal(modal);
  const speedDial = $('#speed-dial-container');
  if (speedDial && !speedDial.contains(event.target)) setSpeedDial(false);
}

function onClick(event) {
  if (runAction(event) || followNavigationLink(event)) return;
  closeOnOutsideClick(event);
}

function onChange(event) {
  if (event.target.id !== 'sort-selector') return;
  const [sort, order] = event.target.value.split(':');
  navigate({ sort, order });
}

function onInput(event) {
  if (event.target.id === 'global-search-input') scheduleSearch(event.target);
}

function focusSearch(event) {
  event.preventDefault();
  $('#global-search-input')?.focus();
}

function dismissTopLayer(event) {
  if (topModal()) {
    closeModal();
    return;
  }
  if (isSpeedDialOpen()) {
    setSpeedDial(false);
    return;
  }
  const isSearchWithText = event.target.id === 'global-search-input' && event.target.value;
  if (isSearchWithText) clearSearch();
}

function onKeyDown(event) {
  const isSearchShortcut = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k';
  const isSlashShortcut = event.key === '/' && !isTypingTarget(event.target);
  if (isSearchShortcut || isSlashShortcut) {
    focusSearch(event);
    return;
  }
  if (event.key === 'Escape') {
    dismissTopLayer(event);
    return;
  }
  trapFocus(event);
}

function onHistoryNavigation() {
  navigate(stateFromLocation(new URL(window.location.href)), { push: false });
}

export function registerActions() {
  document.addEventListener('click', onClick);
  document.addEventListener('change', onChange);
  document.addEventListener('input', onInput);
  document.addEventListener('keydown', onKeyDown);
  window.addEventListener('popstate', onHistoryNavigation);
}
