// Runs synchronously in <head> to apply the saved theme before first paint.
(function () {
  var saved = null;
  try {
    saved = localStorage.getItem('theme');
  } catch (e) {}
  var prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  if (saved === 'dark' || (!saved && prefersDark)) {
    document.documentElement.classList.add('dark');
  }
})();
