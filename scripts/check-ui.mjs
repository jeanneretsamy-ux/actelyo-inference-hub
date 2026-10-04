import { readFileSync, readdirSync } from 'node:fs';
import { Script } from 'node:vm';
import { fileURLToPath } from 'node:url';

const ui = new URL('../internal/actelyohub/ui/', import.meta.url);
for (const name of readdirSync(new URL('src/js/', ui)).filter(name => name.endsWith('.js'))) {
  new Script(readFileSync(new URL(`src/js/${name}`, ui), 'utf8'), { filename: name });
}
const html = readFileSync(new URL('index.html', ui), 'utf8');
const head = html.match(/<head>([\s\S]*?)<\/head>/)?.[1];
if (!head || /<rect\b|<\/svg>/.test(head)) throw new Error('Invalid favicon markup introduces SVG nodes into the document head.');
const scripts = [...html.matchAll(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g)].map(match => match[1]);
if (!scripts.length) throw new Error('No inline scripts found in generated hub UI.');
new Script(scripts.join('\n'), { filename: fileURLToPath(new URL('index.html', ui)) });
console.log('Hub JavaScript sources and generated page parse successfully.');
