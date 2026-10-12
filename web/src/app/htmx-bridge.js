import { $, translate } from './dom.js';
import { pollForChanges, resetChangeWatcher } from './browsing.js';
import { closeModal, rememberFocus } from './modals.js';
import { initImageViewer, renderRichContent } from './preview.js';
import { TOAST_LEVEL, showToast } from './toasts.js';
import { isUploadForm, uploadQueryString } from './uploads.js';
import { currentState, syncChrome } from './view-state.js';

const MAX_DISPLAYED_ERROR_LENGTH = 600;
const MODAL_FORM_IDS = new Set(['folder-form', 'clipboard-form']);
const SERVER_TOAST_EVENT = 'simplefs:toast';

function isModalForm(element) {
  return Boolean(element && MODAL_FORM_IDS.has(element.id));
}

function isEmptyParameter(parameters, key) {
  if (parameters instanceof FormData) return !parameters.has(key) || parameters.get(key) === '';
  return parameters[key] === undefined || parameters[key] === '';
}

function setParameter(parameters, key, value) {
  if (parameters instanceof FormData) parameters.set(key, value);
  else parameters[key] = value;
}

function addViewParameters(parameters, element) {
  const state = currentState();
  const defaults = { view: state.view, sort: state.sort, order: state.order, lang: state.lang };
  if (isModalForm(element)) defaults.path = state.path;
  Object.entries(defaults)
    .filter(([key]) => isEmptyParameter(parameters, key))
    .forEach(([key, value]) => setParameter(parameters, key, value));
}

function onConfigRequest(event) {
  const { elt: element, target, verb, parameters } = event.detail;
  if (isUploadForm(element)) {
    // Streamed uploads read routing parameters from the URL, never from the body.
    event.detail.path = `${event.detail.path.split('?')[0]}?${uploadQueryString()}`;
    return;
  }
  const changesFileList = target && target.id === 'file-list-container' && verb !== 'get';
  if (changesFileList) addViewParameters(parameters, element);
}

function onAfterRequest(event) {
  const form = event.detail.elt;
  if (!event.detail.successful || !isModalForm(form)) return;
  closeModal(form.closest('[data-modal]'));
  form.reset();
}

function readableErrorMessage(xhr) {
  const responseText = xhr?.responseText?.trim() || '';
  const isDisplayable = responseText && responseText.length < MAX_DISPLAYED_ERROR_LENGTH;
  return isDisplayable ? responseText : translate('genericError');
}

function onFileListSwapped() {
  syncChrome();
  resetChangeWatcher();
  pollForChanges();
}

function onModalSwapped(event, modalContainer) {
  document.body.classList.add('overflow-hidden');
  rememberFocus(event.detail.requestConfig?.elt);
  const modal = $('[data-modal]', modalContainer);
  if (modal) $('button, a[href]', modal)?.focus();
  initImageViewer(modalContainer);
}

function onAfterSwap(event) {
  const target = event.detail.target;
  if (target?.id === 'file-list-container') onFileListSwapped();
  if (target?.id === 'modal-container') onModalSwapped(event, target);
  renderRichContent(target);
}

export function registerHtmxBridge() {
  document.addEventListener('htmx:configRequest', onConfigRequest);
  document.addEventListener('htmx:afterRequest', onAfterRequest);
  document.addEventListener('htmx:afterSwap', onAfterSwap);
  document.addEventListener('htmx:responseError', (event) => showToast(TOAST_LEVEL.error, readableErrorMessage(event.detail.xhr)));
  document.addEventListener('htmx:sendError', () => showToast(TOAST_LEVEL.error, translate('networkError')));
  document.addEventListener(SERVER_TOAST_EVENT, (event) => showToast(event.detail.level || TOAST_LEVEL.info, event.detail.message));
}
