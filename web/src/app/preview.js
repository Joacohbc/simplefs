import { $, $$ } from './dom.js';
import { copyText } from './toasts.js';

const MIN_ZOOM = 0.2;
const MAX_ZOOM = 5;
const DOUBLE_CLICK_ZOOM = 2;
const WHEEL_ZOOM_IN_FACTOR = 1.15;
const WHEEL_ZOOM_OUT_FACTOR = 0.87;
const QUARTER_TURN_DEGREES = 90;
const FULL_TURN_DEGREES = 360;
const PERCENT = 100;
const COPIED_FEEDBACK_MS = 1500;
const PRIMARY_MOUSE_BUTTON = 0;

const imageTransform = { scale: 1, offsetX: 0, offsetY: 0, rotationDegrees: 0 };

function renderMarkdown(container) {
  const markdownSource = container.getAttribute('data-markdown');
  const canRenderSafely = window.marked && window.DOMPurify;
  if (!markdownSource || !canRenderSafely) {
    container.textContent = markdownSource || '';
    return;
  }
  container.innerHTML = window.DOMPurify.sanitize(window.marked.parse(markdownSource));
  container.removeAttribute('data-markdown');
  container.dataset.source = markdownSource;
}

function highlightCode(block) {
  const isPending = !block.dataset.highlighted && block.id !== 'mini-preview-code';
  if (window.hljs && isPending) window.hljs.highlightElement(block);
}

export function renderRichContent(root = document) {
  $$('.markdown-content[data-markdown]', root).forEach(renderMarkdown);
  $$('pre code', root).forEach(highlightCode);
}

function applyImageTransform() {
  const image = $('#zoomable-image');
  if (!image) return;
  image.style.setProperty('--scale', imageTransform.scale);
  image.style.setProperty('--tx', `${imageTransform.offsetX}px`);
  image.style.setProperty('--ty', `${imageTransform.offsetY}px`);
  image.style.setProperty('--rotate', `${imageTransform.rotationDegrees}deg`);
  const zoomLabel = $('#image-zoom-label');
  if (zoomLabel) zoomLabel.textContent = `${Math.round(imageTransform.scale * PERCENT)}%`;
}

export function setImageScale(scale) {
  const clampedScale = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, scale));
  imageTransform.scale = Math.round(clampedScale * PERCENT) / PERCENT;
  if (imageTransform.scale === 1) Object.assign(imageTransform, { offsetX: 0, offsetY: 0 });
  applyImageTransform();
}

export function zoomImageBy(deltaPercent) {
  setImageScale(imageTransform.scale + deltaPercent / PERCENT);
}

export function resetImage() {
  Object.assign(imageTransform, { scale: 1, offsetX: 0, offsetY: 0, rotationDegrees: 0 });
  applyImageTransform();
}

export function rotateImage() {
  imageTransform.rotationDegrees = (imageTransform.rotationDegrees + QUARTER_TURN_DEGREES) % FULL_TURN_DEGREES;
  applyImageTransform();
}

function onWheelZoom(event) {
  event.preventDefault();
  const factor = event.deltaY < 0 ? WHEEL_ZOOM_IN_FACTOR : WHEEL_ZOOM_OUT_FACTOR;
  setImageScale(imageTransform.scale * factor);
}

function onDoubleClickZoom() {
  if (imageTransform.scale !== 1) resetImage();
  else setImageScale(DOUBLE_CLICK_ZOOM);
}

function startPanning(viewport, event) {
  if (event.button !== PRIMARY_MOUSE_BUTTON) return;
  const anchorX = event.clientX - imageTransform.offsetX;
  const anchorY = event.clientY - imageTransform.offsetY;
  viewport.setPointerCapture(event.pointerId);
  const pan = (moveEvent) => {
    imageTransform.offsetX = moveEvent.clientX - anchorX;
    imageTransform.offsetY = moveEvent.clientY - anchorY;
    applyImageTransform();
  };
  const stopPanning = () => {
    viewport.removeEventListener('pointermove', pan);
    viewport.removeEventListener('pointerup', stopPanning);
    viewport.removeEventListener('pointercancel', stopPanning);
  };
  viewport.addEventListener('pointermove', pan);
  viewport.addEventListener('pointerup', stopPanning);
  viewport.addEventListener('pointercancel', stopPanning);
}

export function initImageViewer(root) {
  const viewport = $('#image-viewport', root);
  if (!viewport) return;
  resetImage();
  viewport.addEventListener('wheel', onWheelZoom, { passive: false });
  viewport.addEventListener('dblclick', onDoubleClickZoom);
  viewport.addEventListener('pointerdown', (event) => startPanning(viewport, event));
}

function previewSourceText() {
  const codeBlock = $('#preview-modal pre code');
  if (codeBlock) return codeBlock.innerText;
  const markdownViewer = $('#markdown-viewer');
  if (markdownViewer) return markdownViewer.dataset.source || markdownViewer.innerText;
  return '';
}

function showCopiedFeedback(button) {
  const originalContent = button.innerHTML;
  button.replaceChildren($('#icon-check').content.cloneNode(true));
  setTimeout(() => { button.innerHTML = originalContent; }, COPIED_FEEDBACK_MS);
}

export async function copyPreview(button) {
  const sourceText = previewSourceText();
  if (!sourceText) return;
  const wasCopied = await copyText(sourceText);
  if (wasCopied) showCopiedFeedback(button);
}
