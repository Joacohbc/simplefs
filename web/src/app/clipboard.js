import { $, format, translate } from './dom.js';
import { setSpeedDial } from './layout.js';
import { MODAL_CLOSED_EVENT, openModal } from './modals.js';
import { canCreateHere, syncChrome } from './view-state.js';

const MAX_PREVIEW_CHARACTERS = 20000;
const BASE64_BYTES_PER_CHARACTER = 3 / 4;
const BYTES_PER_KILOBYTE = 1024;
const DEFAULT_SNIPPET_NAME = 'snippet';
const IMAGE_EXTENSIONS_BY_TYPE = { 'image/jpeg': '.jpg', 'image/webp': '.webp' };
const DEFAULT_IMAGE_EXTENSION = '.png';
const LANGUAGES_BY_EXTENSION = {
  '.go': 'go', '.js': 'javascript', '.ts': 'typescript', '.py': 'python', '.rs': 'rust',
  '.css': 'css', '.html': 'xml', '.json': 'json', '.sh': 'bash', '.sql': 'sql',
  '.yml': 'yaml', '.yaml': 'yaml', '.md': 'markdown', '.toml': 'ini', '.ini': 'ini',
};

let isImageMode = false;
let pastedImageDataUrl = '';

function setTitle(isImage) {
  const title = $('#clipboard-modal-title');
  if (title) title.textContent = isImage ? translate('titleImage') : translate('titleText');
}

function compactTimestamp() {
  const now = new Date();
  const twoDigits = (value) => String(value).padStart(2, '0');
  const datePart = `${now.getFullYear()}${twoDigits(now.getMonth() + 1)}${twoDigits(now.getDate())}`;
  const timePart = `${twoDigits(now.getHours())}${twoDigits(now.getMinutes())}${twoDigits(now.getSeconds())}`;
  return `${datePart}_${timePart}`;
}

function showImageMode(image) {
  $('#text-input-group').hidden = true;
  $('#image-preview-group').hidden = false;
  $('#mini-preview-text-box').hidden = true;
  setTitle(true);
  image.src = pastedImageDataUrl;
}

function describeImage(image, mimeType) {
  const sizeKilobytes = ((pastedImageDataUrl.length * BASE64_BYTES_PER_CHARACTER) / BYTES_PER_KILOBYTE).toFixed(1);
  $('#mini-preview-stats').textContent = format(translate('statsImage'), image.naturalWidth, image.naturalHeight, mimeType, sizeKilobytes);
}

function loadPastedImage(imageBlob) {
  const reader = new FileReader();
  reader.onload = () => {
    pastedImageDataUrl = reader.result;
    isImageMode = true;
    openClipboardModal({ shouldReadClipboard: false });
    const image = $('#pasted-image-preview');
    image.onload = () => describeImage(image, imageBlob.type);
    showImageMode(image);
    const extension = IMAGE_EXTENSIONS_BY_TYPE[imageBlob.type] || DEFAULT_IMAGE_EXTENSION;
    $('#clipboard-filename').value = `${translate('imagePrefix')}${compactTimestamp()}${extension}`;
    $('#clipboard-is-base64').value = 'true';
    $('#clipboard-hidden-content').value = pastedImageDataUrl;
  };
  reader.readAsDataURL(imageBlob);
}

async function findClipboardImage() {
  if (!navigator.clipboard?.read) return null;
  const items = await navigator.clipboard.read();
  for (const item of items) {
    const imageType = item.types.find((type) => type.startsWith('image/'));
    if (imageType) return item.getType(imageType);
  }
  return null;
}

async function readClipboard() {
  const imageBlob = await findClipboardImage();
  if (imageBlob) {
    loadPastedImage(imageBlob);
    return;
  }
  const pastedText = navigator.clipboard?.readText ? await navigator.clipboard.readText() : '';
  if (!pastedText) return;
  $('#clipboard-content').value = pastedText;
  updateMiniPreview();
}

export function pasteFromClipboard() {
  const allowManualPasteWhenPermissionDenied = () => {};
  readClipboard().catch(allowManualPasteWhenPermissionDenied);
}

export function openClipboardModal({ shouldReadClipboard = true } = {}) {
  setSpeedDial(false);
  syncChrome();
  openModal('clipboard-modal');
  if (shouldReadClipboard && !isImageMode) pasteFromClipboard();
  updateMiniPreview();
}

function resetClipboardForm() {
  isImageMode = false;
  pastedImageDataUrl = '';
  const form = $('#clipboard-form');
  if (!form) return;
  form.reset();
  $('#text-input-group').hidden = false;
  $('#image-preview-group').hidden = true;
  $('#mini-preview-text-box').hidden = false;
  $('#pasted-image-preview').removeAttribute('src');
  $('#clipboard-is-base64').value = 'false';
  $('#clipboard-hidden-content').value = '';
  setTitle(false);
  updateMiniPreview();
}

function filenameWithoutExtension(filename) {
  const lastDot = filename.lastIndexOf('.');
  return lastDot > 0 ? filename.substring(0, lastDot) : filename;
}

export function setExtension(extension) {
  const filenameInput = $('#clipboard-filename');
  const defaultName = isImageMode ? translate('imagePrefix') + compactTimestamp() : DEFAULT_SNIPPET_NAME;
  const baseName = filenameWithoutExtension(filenameInput.value.trim()) || defaultName;
  filenameInput.value = baseName + extension;
  updateMiniPreview();
}

function languageForFilename(filename) {
  const extension = filename.includes('.') ? `.${filename.split('.').pop().toLowerCase()}` : '';
  return LANGUAGES_BY_EXTENSION[extension] || 'plaintext';
}

function renderCodePreview(codeElement, content, filename) {
  delete codeElement.dataset.highlighted;
  if (!content.trim()) {
    codeElement.className = 'language-plaintext';
    codeElement.textContent = translate('previewPlaceholder');
    return;
  }
  codeElement.className = `language-${languageForFilename(filename)}`;
  codeElement.textContent = content.slice(0, MAX_PREVIEW_CHARACTERS);
  window.hljs?.highlightElement(codeElement);
}

export function updateMiniPreview() {
  const stats = $('#mini-preview-stats');
  const codeElement = $('#mini-preview-code');
  if (!stats || !codeElement) return;
  if (isImageMode) {
    $('#clipboard-hidden-content').value = pastedImageDataUrl;
    return;
  }
  const content = $('#clipboard-content').value;
  $('#clipboard-hidden-content').value = content;
  $('#clipboard-is-base64').value = 'false';
  const lineCount = content ? content.split('\n').length : 0;
  stats.textContent = format(translate('statsText'), lineCount, content.length, new Blob([content]).size);
  renderCodePreview(codeElement, content, $('#clipboard-filename').value);
}

function onWindowPaste(event) {
  if (!canCreateHere() || event.target.closest('input, textarea')) return;
  const items = Array.from(event.clipboardData?.items || []);
  const imageFile = items.find((item) => item.type.startsWith('image/'))?.getAsFile();
  if (!imageFile) return;
  event.preventDefault();
  loadPastedImage(imageFile);
}

function onClipboardFieldInput(event) {
  const isClipboardField = event.target.id === 'clipboard-content' || event.target.id === 'clipboard-filename';
  if (isClipboardField) updateMiniPreview();
}

export function registerClipboard() {
  window.addEventListener('paste', onWindowPaste);
  document.addEventListener('input', onClipboardFieldInput);
  document.addEventListener(MODAL_CLOSED_EVENT, (event) => {
    if (event.detail.id === 'clipboard-modal') resetClipboardForm();
  });
}
