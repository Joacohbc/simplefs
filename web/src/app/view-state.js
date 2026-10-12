import { $, $$ } from './dom.js';

const DEFAULT_STATE = { scope: 'dir', view: 'list', sort: 'name', order: 'asc', lang: 'es' };

const SCOPE_ENDPOINTS = {
  dir: { api: '/api/files', page: '/' },
  recent: { api: '/api/recent', page: '/recent' },
  trash: { api: '/api/trash', page: '/trash' },
};

const SCOPE_BY_PATHNAME = { '/trash': 'trash', '/recent': 'recent' };

export function currentState() {
  const activeView = $('#active-view');
  const viewData = activeView ? activeView.dataset : {};
  return {
    scope: viewData.scope || DEFAULT_STATE.scope,
    path: viewData.path || '',
    view: viewData.view || DEFAULT_STATE.view,
    sort: viewData.sort || DEFAULT_STATE.sort,
    order: viewData.order || DEFAULT_STATE.order,
    query: viewData.query || '',
    lang: viewData.lang || document.documentElement.lang || DEFAULT_STATE.lang,
    readOnly: viewData.readonly === 'true',
  };
}

export function canCreateHere(state = currentState()) {
  return state.scope === 'dir' && !state.readOnly;
}

function buildQueryString(state) {
  const isDirectory = state.scope === 'dir';
  const isSortable = state.scope !== 'recent';
  const params = new URLSearchParams();
  if (isDirectory && state.path) params.set('path', state.path);
  if (isDirectory && state.query) params.set('query', state.query);
  if (state.view !== DEFAULT_STATE.view) params.set('view', state.view);
  if (isSortable && state.sort !== DEFAULT_STATE.sort) params.set('sort', state.sort);
  if (isSortable && state.order !== DEFAULT_STATE.order) params.set('order', state.order);
  const queryString = params.toString();
  return queryString ? `?${queryString}` : '';
}

export function navigate(changes = {}, { push = true, replace = false, silent = false } = {}) {
  const state = { ...currentState(), ...changes };
  const endpoints = SCOPE_ENDPOINTS[state.scope] || SCOPE_ENDPOINTS.dir;
  const queryString = buildQueryString(state);
  const request = htmx.ajax('GET', endpoints.api + queryString, {
    target: '#file-list-container',
    swap: 'innerHTML',
    indicator: silent ? undefined : '#global-spinner',
  });
  const pageUrl = endpoints.page + queryString;
  if (push) history.pushState({ simplefs: true }, '', pageUrl);
  if (!push && replace) history.replaceState({ simplefs: true }, '', pageUrl);
  return request;
}

export function stateFromLocation(url) {
  const params = url.searchParams;
  return {
    scope: SCOPE_BY_PATHNAME[url.pathname] || DEFAULT_STATE.scope,
    path: params.get('path') || '',
    query: params.get('query') || '',
    view: params.get('view') || currentState().view,
    sort: params.get('sort') || DEFAULT_STATE.sort,
    order: params.get('order') || DEFAULT_STATE.order,
  };
}

function markCurrentNavigation(scope) {
  $$('[data-scope-link]').forEach((link) => {
    const isCurrent = link.dataset.scopeLink === scope;
    if (isCurrent) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  });
}

export function syncChrome() {
  const state = currentState();
  markCurrentNavigation(state.scope);
  const isCreationHidden = !canCreateHere(state);
  $$('#header-create-actions, #speed-dial-container').forEach((element) => {
    element.hidden = isCreationHidden;
  });
  const searchInput = $('#global-search-input');
  if (searchInput && document.activeElement !== searchInput) searchInput.value = state.query;
  $$('[data-param]').forEach((input) => {
    input.value = state[input.dataset.param] ?? '';
  });
}
