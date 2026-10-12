(function applySavedThemeBeforeFirstPaint() {
  var savedTheme = null;
  try {
    savedTheme = localStorage.getItem('theme');
  } catch (storageUnavailable) {
    savedTheme = null;
  }
  var prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  var shouldUseDark = savedTheme === 'dark' || (!savedTheme && prefersDark);
  if (shouldUseDark) document.documentElement.classList.add('dark');
})();
