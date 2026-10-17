import { $, translate } from './dom.js';
import { TOAST_LEVEL, showToast } from './toasts.js';
import { canCreateHere, currentState } from './view-state.js';

const MIN_VISIBLE_PROGRESS = 0.02;
const UPLOAD_FORM_IDS = new Set(['upload-form', 'upload-zip-form']);

let dragDepth = 0;
let uploadingToast = null;

export function isUploadForm(element) {
  return Boolean(element && UPLOAD_FORM_IDS.has(element.id));
}

export function uploadQueryString() {
  const state = currentState();
  return new URLSearchParams({ path: state.path, view: state.view, sort: state.sort, order: state.order, lang: state.lang }).toString();
}

export function submitUpload(form) {
  htmx.trigger(form, 'submit');
}

function hideProgress(holder, bar) {
  holder.hidden = true;
  bar.style.transform = 'scaleX(0)';
}

function setProgress(fraction) {
  const holder = $('#upload-progress');
  const bar = $('#upload-progress-bar');
  if (!holder || !bar) return;
  if (fraction === null) {
    hideProgress(holder, bar);
    return;
  }
  const visibleFraction = Math.max(MIN_VISIBLE_PROGRESS, Math.min(1, fraction));
  holder.hidden = false;
  bar.style.transform = `scaleX(${visibleFraction})`;
}

function carriesFiles(event) {
  return Boolean(event.dataTransfer) && Array.from(event.dataTransfer.types || []).includes('Files');
}

function onDragEnter(event) {
  if (!carriesFiles(event) || !canCreateHere()) return;
  event.preventDefault();
  dragDepth += 1;
  document.body.classList.add('drag-active');
}

function onDragLeave(event) {
  if (!carriesFiles(event)) return;
  dragDepth = Math.max(0, dragDepth - 1);
  if (!dragDepth) document.body.classList.remove('drag-active');
}

function onDragOver(event) {
  if (carriesFiles(event)) event.preventDefault();
}

function onDrop(event) {
  if (!carriesFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  document.body.classList.remove('drag-active');
  if (!canCreateHere() || !event.dataTransfer.files.length) return;
  $('#global-file-picker').files = event.dataTransfer.files;
  submitUpload($('#upload-form'));
}

function onUploadStarted(event) {
  if (!isUploadForm(event.detail.elt)) return;
  setProgress(0);
  uploadingToast = showToast(TOAST_LEVEL.info, translate('uploading'), { durationMs: 0 });
}

function onUploadProgress(event) {
  const isMeasurable = event.detail.lengthComputable && event.detail.total;
  if (!isUploadForm(event.detail.elt) || !isMeasurable) return;
  setProgress(event.detail.loaded / event.detail.total);
}

function onUploadFinished(event) {
  const form = event.detail.elt;
  if (!isUploadForm(form)) return;
  setProgress(null);
  form.reset();
  uploadingToast?.remove();
  uploadingToast = null;
}

function onFilePicked(event) {
  const picker = event.target;
  if (!picker.files || !picker.files.length) return;
  if (picker.id === 'global-file-picker') submitUpload($('#upload-form'));
  if (picker.id === 'global-zip-picker') submitUpload($('#upload-zip-form'));
}

export function registerUploads() {
  window.addEventListener('dragenter', onDragEnter);
  window.addEventListener('dragleave', onDragLeave);
  window.addEventListener('dragover', onDragOver);
  window.addEventListener('drop', onDrop);
  document.addEventListener('change', onFilePicked);
  document.addEventListener('htmx:beforeRequest', onUploadStarted);
  document.addEventListener('htmx:xhr:progress', onUploadProgress);
  document.addEventListener('htmx:afterRequest', onUploadFinished);
}
