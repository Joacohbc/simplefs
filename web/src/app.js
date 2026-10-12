import { registerActions } from './app/actions.js';
import { startChangeWatcher } from './app/browsing.js';
import { registerClipboard } from './app/clipboard.js';
import { $$ } from './app/dom.js';
import { registerHtmxBridge } from './app/htmx-bridge.js';
import { renderRichContent } from './app/preview.js';
import { registerUploads } from './app/uploads.js';
import { syncChrome } from './app/view-state.js';

const ORIGIN_PLACEHOLDER = '__ORIGIN__';

function fillOriginPlaceholders() {
  $$('[data-origin-template]').forEach((block) => {
    block.textContent = block.textContent.replace(ORIGIN_PLACEHOLDER, window.location.origin);
  });
}

registerActions();
registerUploads();
registerClipboard();
registerHtmxBridge();

document.addEventListener('DOMContentLoaded', () => {
  fillOriginPlaceholders();
  syncChrome();
  renderRichContent();
  startChangeWatcher();
});
