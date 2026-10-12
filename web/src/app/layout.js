import { $, $$ } from './dom.js';

export function isSpeedDialOpen() {
  const container = $('#speed-dial-container');
  return Boolean(container && container.classList.contains('speed-dial-open'));
}

export function setSpeedDial(isOpen) {
  const container = $('#speed-dial-container');
  if (!container) return;
  container.classList.toggle('speed-dial-open', isOpen);
  $('#fab-main-btn')?.setAttribute('aria-expanded', String(isOpen));
  $$('.speed-dial-item button', container).forEach((item) => {
    item.tabIndex = isOpen ? 0 : -1;
  });
}

export function toggleSidebar(shouldOpen) {
  const sidebar = $('#sidebar');
  const backdrop = $('#sidebar-backdrop');
  if (!sidebar || !backdrop) return;
  const isOpening = shouldOpen ?? sidebar.classList.contains('-translate-x-full');
  sidebar.classList.toggle('-translate-x-full', !isOpening);
  backdrop.classList.toggle('hidden', !isOpening);
}
