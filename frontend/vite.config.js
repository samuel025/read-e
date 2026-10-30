import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import fs from 'node:fs';
import path from 'node:path';

// pdf.js needs its cMaps and standard fonts at runtime. Ship them with the app
// under /pdfjs/ so PDFs render correctly without an internet connection.
const pdfjsDir = path.resolve('node_modules/pdfjs-dist');
const pdfjsAssets = ['cmaps', 'standard_fonts'];

function pdfjsAssetsPlugin() {
  let outDir;
  return {
    name: 'pdfjs-assets',
    configResolved(config) {
      outDir = path.resolve(config.root, config.build.outDir);
    },
    configureServer(server) {
      // These routes are served by the Go asset handler. 404 them here so the dev
      // server's SPA fallback does not answer with index.html (Wails then falls
      // through to the Go handler).
      server.middlewares.use((req, res, next) => {
        if (/^\/(cover|epub|pdf)\//.test(req.url)) {
          res.statusCode = 404;
          return res.end();
        }
        next();
      });
      server.middlewares.use('/pdfjs', (req, res, next) => {
        const rel = decodeURIComponent(req.url.split('?')[0]);
        const file = path.join(pdfjsDir, rel);
        if (!file.startsWith(pdfjsDir + path.sep) || !fs.existsSync(file) || !fs.statSync(file).isFile()) {
          return next();
        }
        fs.createReadStream(file).pipe(res);
      });
    },
    writeBundle() {
      for (const dir of pdfjsAssets) {
        fs.cpSync(path.join(pdfjsDir, dir), path.join(outDir, 'pdfjs', dir), { recursive: true });
      }
    },
  };
}

export default defineConfig({
  plugins: [svelte(), pdfjsAssetsPlugin()],
  build: {
    outDir: 'dist',
  },
});
