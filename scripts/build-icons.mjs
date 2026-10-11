// Builds web/static/icons.svg: a sprite with only the Material Symbols the app uses.
// Icon names are collected from the templates and Go sources; a missing icon fails the build.
import { readFileSync, readdirSync, statSync, writeFileSync, mkdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';

const require = createRequire(import.meta.url);
const root = new URL('..', import.meta.url).pathname;
const iconsDir = join(dirname(require.resolve('@material-symbols/svg-400/package.json')), 'outlined');

const patterns = [
  { regex: /\{\{-?\s*icon\s+"([a-z0-9_]+)"/g, fill: false },
  { regex: /\{\{-?\s*iconFill\s+"([a-z0-9_]+)"/g, fill: true },
  { regex: /"Icon"\s+"([a-z0-9_]+)"/g, fill: false },
  { regex: /Icon:\s*"([a-z0-9_]+)"/g, fill: false },
  { regex: /\{"([a-z0-9_]+)",\s*filetype\.Accent/g, fill: false },
  { regex: /MaterialIcon\s*=\s*"([a-z0-9_]+)"/g, fill: false },
];

function walk(directory, extensions, files = []) {
  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry);
    if (statSync(path).isDirectory()) walk(path, extensions, files);
    else if (extensions.some((extension) => path.endsWith(extension)) && !path.endsWith('_test.go')) files.push(path);
  }
  return files;
}

const sources = [
  ...walk(join(root, 'web/templates'), ['.html']),
  ...walk(join(root, 'internal'), ['.go']),
];

const symbols = new Set();
for (const file of sources) {
  const content = readFileSync(file, 'utf8');
  for (const { regex, fill } of patterns) {
    for (const match of content.matchAll(regex)) symbols.add(fill ? `${match[1]}-fill` : match[1]);
  }
}

const missing = [];
const output = [];
for (const symbol of [...symbols].sort()) {
  let svg;
  try {
    svg = readFileSync(join(iconsDir, `${symbol}.svg`), 'utf8');
  } catch {
    missing.push(symbol);
    continue;
  }
  const viewBox = svg.match(/viewBox="([^"]+)"/)[1];
  const body = svg.replace(/^[\s\S]*?<svg[^>]*>/, '').replace(/<\/svg>\s*$/, '').trim();
  output.push(`<symbol id="${symbol}" viewBox="${viewBox}">${body}</symbol>`);
}

if (missing.length) {
  console.error(`Unknown Material Symbols: ${missing.join(', ')}`);
  process.exit(1);
}

mkdirSync(join(root, 'web/static'), { recursive: true });
writeFileSync(
  join(root, 'web/static/icons.svg'),
  `<svg xmlns="http://www.w3.org/2000/svg" style="display:none">${output.join('')}</svg>\n`,
);
console.log(`icons.svg: ${output.length} symbols`);
