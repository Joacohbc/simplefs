import { $, $$ } from './dom.js';

const FOCUS_DELAY_MS = 30;
const FOCUSABLE_SELECTOR = 'a[href], button:not([disabled]), input:not([type=hidden]), textarea, select, [tabindex]:not([tabindex="-1"])';

export const MODAL_CLOSED_EVENT = 'simplefs:modal-closed';

let elementFocusedBeforeModal = null;

export function rememberFocus(element) {
  elementFocusedBeforeModal = element;
}

export function openModal(id) {
  const modal = document.getElementById(id);
  if (!modal) return;
  rememberFocus(document.activeElement);
  modal.hidden = false;
  document.body.classList.add('overflow-hidden');
  const initialFocus = $('[autofocus], input:not([type=hidden]), textarea, button', modal);
  if (initialFocus) setTimeout(() => initialFocus.focus(), FOCUS_DELAY_MS);
}

export function topModal() {
  const openModals = $$('[data-modal]').filter((modal) => !modal.hidden && modal.isConnected);
  return openModals[openModals.length - 1] || null;
}

export function closeModal(modal = topModal()) {
  if (!modal) return;
  const isDynamic = modal.dataset.dynamic !== undefined;
  if (isDynamic) modal.remove();
  else modal.hidden = true;
  document.dispatchEvent(new CustomEvent(MODAL_CLOSED_EVENT, { detail: { id: modal.id } }));
  if (!topModal()) document.body.classList.remove('overflow-hidden');
  if (elementFocusedBeforeModal && elementFocusedBeforeModal.isConnected) elementFocusedBeforeModal.focus();
}

function visibleFocusableElements(modal) {
  return $$(FOCUSABLE_SELECTOR, modal).filter((element) => element.offsetParent !== null);
}

export function trapFocus(event) {
  const modal = topModal();
  if (!modal || event.key !== 'Tab') return;
  const focusable = visibleFocusableElements(modal);
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const isLeavingBackward = event.shiftKey && document.activeElement === first;
  const isLeavingForward = !event.shiftKey && document.activeElement === last;
  if (isLeavingBackward) last.focus();
  if (isLeavingForward) first.focus();
  if (isLeavingBackward || isLeavingForward) event.preventDefault();
}
