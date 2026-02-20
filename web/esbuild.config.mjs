import * as esbuild from 'esbuild';
import { mkdirSync, rmSync, writeFileSync } from 'fs';
import { basename } from 'path';

const outdir = 'static/dist';
const isWatch = process.argv.includes('--watch');

if (!isWatch) {
  rmSync(outdir, { recursive: true, force: true });
}
mkdirSync(outdir, { recursive: true });

const jsEntries = [
  'static/js/src/app.ts',
  'static/js/src/pages/brands.ts',
  'static/js/src/pages/brand-edit.ts',
  'static/js/src/pages/project-detail.ts',
  'static/js/src/pages/work-item-detail.ts',
  'static/js/src/pages/job-detail.ts',
];

const common = {
  outdir,
  bundle: true,
  minify: true,
  sourcemap: false,
  entryNames: '[name]-[hash]',
  metafile: true,
};

const js = await esbuild.build({
  ...common,
  format: 'iife',
  entryPoints: jsEntries,
  ...(isWatch ? { watch: {} } : {}),
});

await esbuild.build({
  outdir,
  bundle: true,
  minify: true,
  sourcemap: false,
  entryNames: '[name]',
  format: 'iife',
  entryPoints: jsEntries,
  ...(isWatch ? { watch: {} } : {}),
});

const css = await esbuild.build({
  ...common,
  entryPoints: [
    'static/css/src/app.css',
  ],
  ...(isWatch ? { watch: {} } : {}),
});

await esbuild.build({
  outdir,
  bundle: true,
  minify: true,
  sourcemap: false,
  entryNames: '[name]',
  entryPoints: [
    'static/css/src/app.css',
  ],
  ...(isWatch ? { watch: {} } : {}),
});

const manifest = {};

for (const [outputPath] of Object.entries(js.metafile.outputs)) {
  const file = basename(outputPath);
  const m = file.match(/^(.+?)-[A-Z0-9]+\.js$/i);
  if (m) {
    manifest[`${m[1]}.js`] = file;
  }
}

for (const [outputPath] of Object.entries(css.metafile.outputs)) {
  const file = basename(outputPath);
  const m = file.match(/^(.+?)-[A-Z0-9]+\.css$/i);
  if (m) {
    manifest[`${m[1]}.css`] = file;
  }
}

writeFileSync(`${outdir}/manifest.json`, JSON.stringify(manifest, null, 2));
console.log('Build complete. Manifest:', manifest);
