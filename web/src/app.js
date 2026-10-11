// SimpleFS client logic. Every interaction is wired through data-action attributes
// and event delegation so the page runs under a strict CSP (no inline scripts).

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

const STAMP_INTERVAL_MS = 5000;
const SEARCH_DEBOUNCE_MS = 250;
const CATEGORY_GROUPS = {
  code: ['code', 'markdown'],
  docs: ['pdf', 'binary', 'markdown'],
  media: ['image', 'video', 'audio'],
};
const SCOPE_ENDPOINTS = {
  dir: { api: '/api/files', page: '/' },
  recent: { api: '/api/recent', page: '/recent' },
  trash: { api: '/api/trash', page: '/trash' },
};
const CLIPBOARD_LANGUAGES = {
  '.go': 'go', '.js': 'javascript', '.ts': 'typescript', '.py': 'python', '.rs': 'rust',
  '.css': 'css', '.html': 'xml', '.json': 'json', '.sh': 'bash', '.sql': 'sql',
  '.yml': 'yaml', '.yaml': 'yaml', '.md': 'markdown', '.toml': 'ini', '.ini': 'ini',
};

function text(key) {
  const holder = $('#i18n');
  return (holder && holder.dataset[key]) || '';
}

function format(template, ...values) {
  let index = 0;
  return template.replace(/%[sd]/g, () => String(values[index++] ?? ''));
}

// ---------------------------------------------------------------------------
// View state
// ---------------------------------------------------------------------------

function currentState() {
  const view = $('#active-view');
  const data = view ? view.dataset : {};
  return {
    scope: data.scope || 'dir',
    path: data.path || '',
    view: data.view || 'list',
    sort: data.sort || 'name',
    order: data.order || 'asc',
    query: data.query || '',
    lang: data.lang || document.documentElement.lang || 'es',
    readOnly: data.readonly === 'true',
  };
}

function buildQuery(state) {
  const params = new URLSearchParams();
  if (state.scope === 'dir' && state.path) params.set('path', state.path);
  if (state.scope === 'dir' && state.query) params.set('query', state.query);
  if (state.view && state.view !== 'list') params.set('view', state.view);
  if (state.scope !== 'recent') {
    if (state.sort && state.sort !== 'name') params.set('sort', state.sort);
    if (state.order && state.order !== 'asc') params.set('order', state.order);
  }
  const query = params.toString();
  return query ? `?${query}` : '';
}

function navigate(changes = {}, { push = true, replace = false, silent = false } = {}) {
  const state = { ...currentState(), ...changes };
  const endpoints = SCOPE_ENDPOINTS[state.scope] || SCOPE_ENDPOINTS.dir;
  const query = buildQuery(state);
  const request = htmx.ajax('GET', endpoints.api + query, {
    target: '#file-list-container',
    swap: 'innerHTML',
    indicator: silent ? undefined : '#global-spinner',
  });
  const pageUrl = endpoints.page + query;
  if (push) history.pushState({ simplefs: true }, '', pageUrl);
  else if (replace) history.replaceState({ simplefs: true }, '', pageUrl);
  return request;
}

function stateFromLocation(url) {
  const params = url.searchParams;
  let scope = 'dir';
  if (url.pathname === '/trash') scope = 'trash';
  else if (url.pathname === '/recent') scope = 'recent';
  return {
    scope,
    path: params.get('path') || '',
    query: params.get('query') || '',
    view: params.get('view') || currentState().view,
    sort: params.get('sort') || 'name',
    order: params.get('order') || 'asc',
  };
}

function syncChrome() {
  const state = currentState();
  $$('[data-scope-link]').forEach((link) => {
    if (link.dataset.scopeLink === state.scope) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  });
  const canCreate = state.scope === 'dir' && !state.readOnly;
  const createActions = $('#header-create-actions');
  if (createActions) createActions.hidden = !canCreate;
  const speedDial = $('#speed-dial-container');
  if (speedDial) speedDial.hidden = !canCreate;
  const search = $('#global-search-input');
  if (search && document.activeElement !== search) search.value = state.query;
  $$('[data-param]').forEach((input) => {
    input.value = state[input.dataset.param] ?? '';
  });
}

// ---------------------------------------------------------------------------
// Toasts and feedback
// ---------------------------------------------------------------------------

function showToast(level, message, { timeout = 4500 } = {}) {
  const region = $('#toast-region');
  const template = $('#toast-template');
  if (!region || !template || !message) return null;
  const toast = template.content.firstElementChild.cloneNode(true);
  const iconTemplate = $(`#toast-icon-${level}`) || $('#toast-icon-info');
  $('.toast-icon', toast).append(iconTemplate.content.cloneNode(true));
  $('.toast-message', toast).textContent = message;
  if (level === 'error') toast.setAttribute('role', 'alert');
  const remove = () => toast.remove();
  $('.toast-close', toast).addEventListener('click', remove);
  region.append(toast);
  while (region.children.length > 4) region.firstElementChild.remove();
  if (timeout) setTimeout(remove, level === 'error' ? timeout * 2 : timeout);
  return toast;
}

async function copyText(value) {
  try {
    await navigator.clipboard.writeText(value);
    showToast('success', text('copied'));
    return true;
  } catch (error) {
    showToast('error', text('copyFailed'));
    return false;
  }
}

function encodePath(path) {
  return path.split('/').filter(Boolean).map(encodeURIComponent).join('/');
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

// ---------------------------------------------------------------------------
// Modals
// ---------------------------------------------------------------------------

let lastFocused = null;

function openModal(id) {
  const modal = document.getElementById(id);
  if (!modal) return;
  lastFocused = document.activeElement;
  modal.hidden = false;
  document.body.classList.add('overflow-hidden');
  const focusTarget = $('[autofocus], input:not([type=hidden]), textarea, button', modal);
  if (focusTarget) setTimeout(() => focusTarget.focus(), 30);
}

function topModal() {
  const open = $$('[data-modal]').filter((modal) => !modal.hidden && modal.isConnected);
  return open[open.length - 1] || null;
}

function closeModal(modal = topModal()) {
  if (!modal) return;
  if (modal.dataset.dynamic !== undefined) modal.remove();
  else modal.hidden = true;
  if (modal.id === 'clipboard-modal') resetClipboardForm();
  if (!topModal()) document.body.classList.remove('overflow-hidden');
  if (lastFocused && lastFocused.isConnected) lastFocused.focus();
}

function trapFocus(event) {
  const modal = topModal();
  if (!modal || event.key !== 'Tab') return;
  const focusable = $$('a[href], button:not([disabled]), input:not([type=hidden]), textarea, select, [tabindex]:not([tabindex="-1"])', modal)
    .filter((element) => element.offsetParent !== null);
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    last.focus();
    event.preventDefault();
  } else if (!event.shiftKey && document.activeElement === last) {
    first.focus();
    event.preventDefault();
  }
}

// ---------------------------------------------------------------------------
// Speed dial and sidebar
// ---------------------------------------------------------------------------

function setSpeedDial(open) {
  const container = $('#speed-dial-container');
  const button = $('#fab-main-btn');
  if (!container) return;
  container.classList.toggle('speed-dial-open', open);
  if (button) button.setAttribute('aria-expanded', String(open));
  $$('.speed-dial-item button', container).forEach((item) => {
    item.tabIndex = open ? 0 : -1;
  });
}

function toggleSidebar(force) {
  const sidebar = $('#sidebar');
  const backdrop = $('#sidebar-backdrop');
  if (!sidebar || !backdrop) return;
  const open = force ?? sidebar.classList.contains('-translate-x-full');
  sidebar.classList.toggle('-translate-x-full', !open);
  backdrop.classList.toggle('hidden', !open);
}

// ---------------------------------------------------------------------------
// Uploads, drag and drop
// ---------------------------------------------------------------------------

function uploadQuery() {
  const state = currentState();
  const params = new URLSearchParams({ path: state.path, view: state.view, sort: state.sort, order: state.order, lang: state.lang });
  return params.toString();
}

function submitUpload(form) {
  htmx.trigger(form, 'submit');
}

function setProgress(fraction) {
  const holder = $('#upload-progress');
  const bar = $('#upload-progress-bar');
  if (!holder || !bar) return;
  if (fraction === null) {
    holder.hidden = true;
    bar.style.transform = 'scaleX(0)';
    return;
  }
  holder.hidden = false;
  bar.style.transform = `scaleX(${Math.max(0.02, Math.min(1, fraction))})`;
}

let dragDepth = 0;

function hasFiles(event) {
  return event.dataTransfer && Array.from(event.dataTransfer.types || []).includes('Files');
}

function canUploadHere() {
  const state = currentState();
  return state.scope === 'dir' && !state.readOnly;
}

// ---------------------------------------------------------------------------
// Clipboard modal
// ---------------------------------------------------------------------------

let clipboardImageMode = false;
let pastedDataUrl = '';

function setClipboardTitle(isImage) {
  const title = $('#clipboard-modal-title');
  if (title) title.textContent = isImage ? text('titleImage') : text('titleText');
}

function timestamp() {
  const now = new Date();
  const pad = (value) => String(value).padStart(2, '0');
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}_${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
}

function handlePastedImage(blob) {
  const reader = new FileReader();
  reader.onload = () => {
    pastedDataUrl = reader.result;
    clipboardImageMode = true;
    openClipboardModal({ readClipboard: false });
    $('#text-input-group').hidden = true;
    $('#image-preview-group').hidden = false;
    $('#mini-preview-text-box').hidden = true;
    setClipboardTitle(true);
    const image = $('#pasted-image-preview');
    image.src = pastedDataUrl;
    const extension = blob.type === 'image/jpeg' ? '.jpg' : blob.type === 'image/webp' ? '.webp' : '.png';
    $('#clipboard-filename').value = `${text('imagePrefix')}${timestamp()}${extension}`;
    $('#clipboard-is-base64').value = 'true';
    $('#clipboard-hidden-content').value = pastedDataUrl;
    image.onload = () => {
      const kilobytes = (Math.round((pastedDataUrl.length * 3) / 4) / 1024).toFixed(1);
      $('#mini-preview-stats').textContent = format(text('statsImage'), image.naturalWidth, image.naturalHeight, blob.type, kilobytes);
    };
  };
  reader.readAsDataURL(blob);
}

async function pasteFromClipboard() {
  try {
    if (navigator.clipboard && navigator.clipboard.read) {
      const items = await navigator.clipboard.read();
      for (const item of items) {
        const imageType = item.types.find((type) => type.startsWith('image/'));
        if (imageType) {
          handlePastedImage(await item.getType(imageType));
          return;
        }
      }
    }
    if (navigator.clipboard && navigator.clipboard.readText) {
      const value = await navigator.clipboard.readText();
      if (value) {
        $('#clipboard-content').value = value;
        updateMiniPreview();
      }
    }
  } catch (error) {
    // Clipboard permission denied: the user can still paste manually.
  }
}

function openClipboardModal({ readClipboard = true } = {}) {
  setSpeedDial(false);
  syncChrome();
  openModal('clipboard-modal');
  if (readClipboard && !clipboardImageMode) pasteFromClipboard();
  updateMiniPreview();
}

function resetClipboardForm() {
  clipboardImageMode = false;
  pastedDataUrl = '';
  const form = $('#clipboard-form');
  if (!form) return;
  form.reset();
  $('#text-input-group').hidden = false;
  $('#image-preview-group').hidden = true;
  $('#mini-preview-text-box').hidden = false;
  $('#pasted-image-preview').removeAttribute('src');
  $('#clipboard-is-base64').value = 'false';
  $('#clipboard-hidden-content').value = '';
  setClipboardTitle(false);
  updateMiniPreview();
}

function setExtension(extension) {
  const input = $('#clipboard-filename');
  let base = input.value.trim();
  const dot = base.lastIndexOf('.');
  if (dot > 0) base = base.substring(0, dot);
  if (!base) base = clipboardImageMode ? text('imagePrefix') + timestamp() : 'snippet';
  input.value = base + extension;
  updateMiniPreview();
}

function updateMiniPreview() {
  const stats = $('#mini-preview-stats');
  const code = $('#mini-preview-code');
  if (!stats || !code) return;
  if (clipboardImageMode) {
    $('#clipboard-hidden-content').value = pastedDataUrl;
    return;
  }
  const content = $('#clipboard-content').value;
  const filename = $('#clipboard-filename').value;
  $('#clipboard-hidden-content').value = content;
  $('#clipboard-is-base64').value = 'false';
  const lines = content ? content.split('\n').length : 0;
  stats.textContent = format(text('statsText'), lines, content.length, new Blob([content]).size);

  delete code.dataset.highlighted;
  if (!content.trim()) {
    code.className = 'language-plaintext';
    code.textContent = text('previewPlaceholder');
    return;
  }
  const extension = filename.includes('.') ? `.${filename.split('.').pop().toLowerCase()}` : '';
  const language = CLIPBOARD_LANGUAGES[extension] || 'plaintext';
  code.className = `language-${language}`;
  code.textContent = content.length > 20000 ? content.slice(0, 20000) : content;
  if (window.hljs) window.hljs.highlightElement(code);
}

// ---------------------------------------------------------------------------
// Rich content: markdown, syntax highlighting, image viewer
// ---------------------------------------------------------------------------

function renderRichContent(root = document) {
  $$('.markdown-content[data-markdown]', root).forEach((container) => {
    const raw = container.getAttribute('data-markdown');
    if (!raw || !window.marked || !window.DOMPurify) {
      container.textContent = raw || '';
      return;
    }
    container.innerHTML = window.DOMPurify.sanitize(window.marked.parse(raw));
    container.removeAttribute('data-markdown');
    container.dataset.source = raw;
  });
  $$('pre code', root).forEach((block) => {
    if (window.hljs && !block.dataset.highlighted && block.id !== 'mini-preview-code') {
      window.hljs.highlightElement(block);
    }
  });
}

const imageState = { scale: 1, x: 0, y: 0, rotation: 0 };

function applyImageTransform() {
  const image = $('#zoomable-image');
  if (!image) return;
  image.style.setProperty('--scale', imageState.scale);
  image.style.setProperty('--tx', `${imageState.x}px`);
  image.style.setProperty('--ty', `${imageState.y}px`);
  image.style.setProperty('--rotate', `${imageState.rotation}deg`);
  const label = $('#image-zoom-label');
  if (label) label.textContent = `${Math.round(imageState.scale * 100)}%`;
}

function setImageScale(scale) {
  imageState.scale = Math.round(Math.max(0.2, Math.min(5, scale)) * 100) / 100;
  if (imageState.scale === 1) {
    imageState.x = 0;
    imageState.y = 0;
  }
  applyImageTransform();
}

function resetImage() {
  Object.assign(imageState, { scale: 1, x: 0, y: 0, rotation: 0 });
  applyImageTransform();
}

function initImageViewer(root) {
  const viewport = $('#image-viewport', root);
  if (!viewport) return;
  resetImage();
  viewport.addEventListener('wheel', (event) => {
    event.preventDefault();
    setImageScale(imageState.scale * (event.deltaY < 0 ? 1.15 : 0.87));
  }, { passive: false });
  viewport.addEventListener('dblclick', () => (imageState.scale !== 1 ? resetImage() : setImageScale(2)));
  viewport.addEventListener('pointerdown', (event) => {
    if (event.button !== 0) return;
    const startX = event.clientX - imageState.x;
    const startY = event.clientY - imageState.y;
    viewport.setPointerCapture(event.pointerId);
    const move = (moveEvent) => {
      imageState.x = moveEvent.clientX - startX;
      imageState.y = moveEvent.clientY - startY;
      applyImageTransform();
    };
    const up = () => {
      viewport.removeEventListener('pointermove', move);
      viewport.removeEventListener('pointerup', up);
      viewport.removeEventListener('pointercancel', up);
    };
    viewport.addEventListener('pointermove', move);
    viewport.addEventListener('pointerup', up);
    viewport.addEventListener('pointercancel', up);
  });
}

function copyPreview(button) {
  const code = $('#preview-modal pre code');
  const markdown = $('#markdown-viewer');
  const value = code ? code.innerText : markdown ? markdown.dataset.source || markdown.innerText : '';
  if (!value) return;
  copyText(value).then((ok) => {
    if (!ok) return;
    const original = button.innerHTML;
    button.replaceChildren($('#icon-check').content.cloneNode(true));
    setTimeout(() => { button.innerHTML = original; }, 1500);
  });
}

// ---------------------------------------------------------------------------
// Filters, search and auto refresh
// ---------------------------------------------------------------------------

function filterByCategory(category, button) {
  $$('#filter-container .chip').forEach((chip) => chip.setAttribute('aria-pressed', String(chip === button)));
  $$('.file-item').forEach((item) => {
    const itemCategory = item.dataset.category || '';
    const visible = category === 'all' || (CATEGORY_GROUPS[category] || []).includes(itemCategory);
    item.hidden = !visible;
  });
}

let searchTimer = null;

function onSearchInput(input) {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    const state = currentState();
    const changes = { query: input.value.trim() };
    if (state.scope !== 'dir') Object.assign(changes, { scope: 'dir', path: '' });
    navigate(changes, { push: false, replace: true });
  }, SEARCH_DEBOUNCE_MS);
}

let lastStamp = null;
let stampKey = '';
let stampBusy = false;

function stampURL() {
  const state = currentState();
  const params = new URLSearchParams({ scope: state.scope, path: state.path });
  return `/api/v1/stamp?${params}`;
}

async function pollStamp() {
  if (stampBusy || document.hidden || topModal() || document.body.classList.contains('htmx-request')) return;
  const key = stampURL();
  stampBusy = true;
  try {
    const response = await fetch(key, { headers: { Accept: 'application/json' }, credentials: 'same-origin' });
    if (!response.ok) return;
    const { stamp } = await response.json();
    if (key !== stampKey) {
      stampKey = key;
      lastStamp = stamp;
      return;
    }
    if (lastStamp !== null && stamp !== lastStamp) {
      lastStamp = stamp;
      const scrollY = window.scrollY;
      await navigate({}, { push: false, silent: true });
      window.scrollTo(0, scrollY);
    }
    lastStamp = stamp;
  } catch (error) {
    // Offline or server restarting: try again on the next tick.
  } finally {
    stampBusy = false;
  }
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

const actions = {
  'toggle-theme'() {
    const dark = document.documentElement.classList.toggle('dark');
    try { localStorage.setItem('theme', dark ? 'dark' : 'light'); } catch (error) { /* private mode */ }
  },
  'set-lang'(element) {
    const url = new URL(window.location.href);
    url.searchParams.set('lang', element.dataset.lang);
    window.location.href = url.toString();
  },
  'toggle-sidebar'() { toggleSidebar(); },
  'toggle-speed-dial'() {
    setSpeedDial(!$('#speed-dial-container').classList.contains('speed-dial-open'));
  },
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
    document.cookie = `view=${view}; path=/; max-age=31536000; SameSite=Lax`;
    navigate({ view });
  },
  'column-sort'(element) {
    const state = currentState();
    const sort = element.dataset.sort;
    const order = state.sort === sort && state.order === 'asc' ? 'desc' : 'asc';
    navigate({ sort, order });
  },
  'filter'(element) { filterByCategory(element.dataset.category, element); },
  'clear-search'() {
    const input = $('#global-search-input');
    if (input) input.value = '';
    navigate({ query: '' });
  },
  'copy-curl'(element) {
    const url = `${window.location.origin}/api/v1/files/${encodePath(element.dataset.path)}`;
    copyText(`curl -fL -H "Authorization: Bearer $SIMPLEFS_TOKEN" -o ${shellQuote(element.dataset.name)} ${shellQuote(url)}`);
  },
  'copy-link'(element) {
    copyText(`${window.location.origin}/download?path=${encodeURIComponent(element.dataset.path)}&download=1`);
  },
  'copy-code'(element) {
    const block = element.closest('li').querySelector('pre');
    if (block) copyText(block.textContent);
  },
  'copy-preview'(element) { copyPreview(element); },
  'print'() { window.print(); },
  'img-zoom'(element) { setImageScale(imageState.scale + Number(element.dataset.delta) / 100); },
  'img-reset'() { resetImage(); },
  'img-rotate'() {
    imageState.rotation = (imageState.rotation + 90) % 360;
    applyImageTransform();
  },
};

document.addEventListener('click', (event) => {
  const actionElement = event.target.closest('[data-action]');
  if (actionElement && actionElement.tagName !== 'SELECT' && actions[actionElement.dataset.action]) {
    event.preventDefault();
    actions[actionElement.dataset.action](actionElement, event);
    return;
  }

  const link = event.target.closest('a[data-nav]');
  if (link && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey && event.button === 0) {
    event.preventDefault();
    toggleSidebar(false);
    const target = stateFromLocation(new URL(link.href));
    navigate({ ...target, view: currentState().view, query: '' });
    return;
  }

  const modal = event.target.closest('[data-modal]');
  if (modal && event.target === modal) closeModal(modal);

  const speedDial = $('#speed-dial-container');
  if (speedDial && !speedDial.contains(event.target)) setSpeedDial(false);
});

document.addEventListener('change', (event) => {
  const target = event.target;
  if (target.id === 'sort-selector') {
    const [sort, order] = target.value.split(':');
    navigate({ sort, order });
  } else if (target.id === 'global-file-picker' && target.files.length) {
    submitUpload($('#upload-form'));
  } else if (target.id === 'global-zip-picker' && target.files.length) {
    submitUpload($('#upload-zip-form'));
  }
});

document.addEventListener('input', (event) => {
  if (event.target.id === 'global-search-input') onSearchInput(event.target);
  if (event.target.id === 'clipboard-content' || event.target.id === 'clipboard-filename') updateMiniPreview();
});

document.addEventListener('keydown', (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    $('#global-search-input')?.focus();
    return;
  }
  if (event.key === '/' && !event.target.closest('input, textarea, select, [contenteditable]')) {
    event.preventDefault();
    $('#global-search-input')?.focus();
    return;
  }
  if (event.key === 'Escape') {
    if (topModal()) {
      closeModal();
    } else if ($('#speed-dial-container')?.classList.contains('speed-dial-open')) {
      setSpeedDial(false);
    } else if (event.target.id === 'global-search-input' && event.target.value) {
      actions['clear-search']();
    }
    return;
  }
  trapFocus(event);
});

window.addEventListener('paste', (event) => {
  if (!canUploadHere() || event.target.closest('input, textarea')) return;
  const items = Array.from((event.clipboardData && event.clipboardData.items) || []);
  const image = items.find((item) => item.type.startsWith('image/'));
  if (image) {
    const file = image.getAsFile();
    if (file) {
      event.preventDefault();
      handlePastedImage(file);
    }
  }
});

window.addEventListener('dragenter', (event) => {
  if (!hasFiles(event) || !canUploadHere()) return;
  event.preventDefault();
  dragDepth += 1;
  document.body.classList.add('drag-active');
});
window.addEventListener('dragleave', (event) => {
  if (!hasFiles(event)) return;
  dragDepth = Math.max(0, dragDepth - 1);
  if (!dragDepth) document.body.classList.remove('drag-active');
});
window.addEventListener('dragover', (event) => {
  if (hasFiles(event)) event.preventDefault();
});
window.addEventListener('drop', (event) => {
  if (!hasFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  document.body.classList.remove('drag-active');
  if (!canUploadHere() || !event.dataTransfer.files.length) return;
  const picker = $('#global-file-picker');
  picker.files = event.dataTransfer.files;
  submitUpload($('#upload-form'));
});

window.addEventListener('popstate', () => {
  navigate(stateFromLocation(new URL(window.location.href)), { push: false });
});

// ---------------------------------------------------------------------------
// htmx integration
// ---------------------------------------------------------------------------

document.addEventListener('htmx:configRequest', (event) => {
  const element = event.detail.elt;
  if (element && (element.id === 'upload-form' || element.id === 'upload-zip-form')) {
    // Streamed uploads read routing parameters from the URL, never from the body.
    event.detail.path = `${event.detail.path.split('?')[0]}?${uploadQuery()}`;
    return;
  }
  if (event.detail.target && event.detail.target.id === 'file-list-container' && event.detail.verb !== 'get') {
    const state = currentState();
    const parameters = event.detail.parameters;
    const setDefault = (key, value) => {
      if (parameters instanceof FormData) {
        if (!parameters.has(key) || parameters.get(key) === '') parameters.set(key, value);
      } else if (parameters[key] === undefined || parameters[key] === '') {
        parameters[key] = value;
      }
    };
    setDefault('view', state.view);
    setDefault('sort', state.sort);
    setDefault('order', state.order);
    setDefault('lang', state.lang);
    if (element && (element.id === 'folder-form' || element.id === 'clipboard-form')) setDefault('path', state.path);
  }
});

document.addEventListener('htmx:xhr:progress', (event) => {
  const element = event.detail.elt;
  if (!element || !(element.id === 'upload-form' || element.id === 'upload-zip-form')) return;
  if (event.detail.lengthComputable && event.detail.total) setProgress(event.detail.loaded / event.detail.total);
});

let uploadToast = null;

document.addEventListener('htmx:beforeRequest', (event) => {
  const element = event.detail.elt;
  if (element && (element.id === 'upload-form' || element.id === 'upload-zip-form')) {
    setProgress(0);
    uploadToast = showToast('info', text('uploading'), { timeout: 0 });
  }
});

document.addEventListener('htmx:afterRequest', (event) => {
  const element = event.detail.elt;
  if (element && (element.id === 'upload-form' || element.id === 'upload-zip-form')) {
    setProgress(null);
    element.reset();
    if (uploadToast) uploadToast.remove();
    uploadToast = null;
  }
  if (event.detail.successful && element && (element.id === 'folder-form' || element.id === 'clipboard-form')) {
    closeModal(element.closest('[data-modal]'));
    element.reset();
  }
});

document.addEventListener('htmx:responseError', (event) => {
  const xhr = event.detail.xhr;
  const message = (xhr && xhr.responseText && xhr.responseText.length < 600 && xhr.responseText.trim()) || text('genericError');
  showToast('error', message);
});

document.addEventListener('htmx:sendError', () => showToast('error', text('networkError')));

document.addEventListener('simplefs:toast', (event) => {
  showToast(event.detail.level || 'info', event.detail.message);
});

document.addEventListener('htmx:afterSwap', (event) => {
  const target = event.detail.target;
  if (target && target.id === 'file-list-container') {
    syncChrome();
    stampKey = '';
    pollStamp();
  }
  if (target && target.id === 'modal-container') {
    document.body.classList.add('overflow-hidden');
    lastFocused = event.detail.requestConfig && event.detail.requestConfig.elt;
    const modal = $('[data-modal]', target);
    if (modal) $('button, a[href]', modal)?.focus();
    initImageViewer(target);
  }
  renderRichContent(target);
});

document.addEventListener('DOMContentLoaded', () => {
  $$('[data-origin-template]').forEach((block) => {
    block.textContent = block.textContent.replace('__ORIGIN__', window.location.origin);
  });
  syncChrome();
  renderRichContent();
  pollStamp();
  setInterval(pollStamp, STAMP_INTERVAL_MS);
});
