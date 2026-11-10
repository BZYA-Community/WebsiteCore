import assert from 'node:assert/strict';
import fs from 'node:fs';
import { compileTemplate, parse } from '@vue/compiler-sfc';
import * as Vue from 'vue';

const source = fs.readFileSync(new URL('../src/components/project-card.vue', import.meta.url), 'utf8');
const { descriptor } = parse(source);
const compiled = compileTemplate({ source: descriptor.template.content, filename: 'project-card.vue', id: 'cover-test' });
assert.deepEqual(compiled.errors, []);
const code = compiled.code.replace(/import \{([^}]+)\} from "vue"/, (_, imports) =>
  'const {' + imports.replace(/ as /g, ': ') + '} = Vue;').replace('export function render', 'function render');
const render = new Function('Vue', code + '; return render;')(Vue);
function walk(node, predicate, results = []) {
  if (!node || typeof node !== 'object') return results;
  if (predicate(node)) results.push(node);
  if (Array.isArray(node.children)) for (const child of node.children) walk(child, predicate, results);
  return results;
}
for (const count of [0, 1, 2, 4, 6]) {
  const images = Array.from({ length: count }, (_, i) => 'https://example.test/' + i + '.png');
  const tree = render({
    project: { images, title: 'Cover title', creator: {}, tags: [], intro: '' },
    t: (key) => key, prepareMdRender: (value) => value,
  }, []);
  const gallery = walk(tree, (node) => node.props?.class === 'project-gallery')[0];
  const covers = walk(gallery, (node) => node.type === 'n-image');
  assert.equal(covers.length, count ? 1 : 0);
  if (count) {
    assert.equal(covers[0].props.src, images[0]);
    assert.equal(covers[0].props.alt, 'Cover title');
    assert.equal(covers[0].props['object-fit'], 'cover');
  }
}
console.log('Project card cover rendering passed for 0, 1, 2, 4 and 6 images.');
