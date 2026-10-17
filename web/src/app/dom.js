export const $ = (selector, root = document) => root.querySelector(selector);
export const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

export function translate(key) {
  const translations = $('#i18n');
  return (translations && translations.dataset[key]) || '';
}

export function format(template, ...values) {
  let valueIndex = 0;
  return template.replace(/%[sd]/g, () => String(values[valueIndex++] ?? ''));
}

export function isTypingTarget(element) {
  return Boolean(element.closest('input, textarea, select, [contenteditable]'));
}
