import { $$ } from './dom.js';
import { topModal } from './modals.js';
import { currentState, navigate } from './view-state.js';

const SEARCH_DEBOUNCE_MS = 250;
const STAMP_POLL_INTERVAL_MS = 5000;
const CATEGORY_GROUPS = {
  code: ['code', 'markdown'],
  docs: ['pdf', 'binary', 'markdown'],
  media: ['image', 'video', 'audio'],
};
const ALL_CATEGORIES = 'all';

export function filterByCategory(category, selectedChip) {
  $$('#filter-container .chip').forEach((chip) => chip.setAttribute('aria-pressed', String(chip === selectedChip)));
  const visibleCategories = CATEGORY_GROUPS[category] || [];
  $$('.file-item').forEach((item) => {
    const isVisible = category === ALL_CATEGORIES || visibleCategories.includes(item.dataset.category || '');
    item.hidden = !isVisible;
  });
}

let searchDebounceTimer = null;

export function scheduleSearch(searchInput) {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    const changes = { query: searchInput.value.trim() };
    const isOutsideFolders = currentState().scope !== 'dir';
    if (isOutsideFolders) Object.assign(changes, { scope: 'dir', path: '' });
    navigate(changes, { push: false, replace: true });
  }, SEARCH_DEBOUNCE_MS);
}

const stampWatcher = { url: '', lastStamp: null, isPolling: false };

function currentStampUrl() {
  const state = currentState();
  return `/api/v1/stamp?${new URLSearchParams({ scope: state.scope, path: state.path })}`;
}

function shouldSkipPolling() {
  const isRequestInFlight = document.body.classList.contains('htmx-request');
  return stampWatcher.isPolling || document.hidden || Boolean(topModal()) || isRequestInFlight;
}

async function fetchStamp(url) {
  const response = await fetch(url, { headers: { Accept: 'application/json' }, credentials: 'same-origin' });
  if (!response.ok) return null;
  const { stamp } = await response.json();
  return stamp;
}

async function refreshKeepingScroll() {
  const scrollY = window.scrollY;
  await navigate({}, { push: false, silent: true });
  window.scrollTo(0, scrollY);
}

async function checkForChanges() {
  const url = currentStampUrl();
  const stamp = await fetchStamp(url);
  if (stamp === null) return;
  const isSameView = url === stampWatcher.url;
  const hasChanged = isSameView && stampWatcher.lastStamp !== null && stamp !== stampWatcher.lastStamp;
  stampWatcher.url = url;
  stampWatcher.lastStamp = stamp;
  if (hasChanged) await refreshKeepingScroll();
}

export async function pollForChanges() {
  if (shouldSkipPolling()) return;
  stampWatcher.isPolling = true;
  const retryOnNextTick = () => {};
  await checkForChanges().catch(retryOnNextTick);
  stampWatcher.isPolling = false;
}

export function resetChangeWatcher() {
  stampWatcher.url = '';
  stampWatcher.lastStamp = null;
}

export function startChangeWatcher() {
  pollForChanges();
  setInterval(pollForChanges, STAMP_POLL_INTERVAL_MS);
}
