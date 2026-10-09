import { test } from 'node:test';
import assert from 'node:assert/strict';
import { selectorFor } from './selector.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

function el(tag, { id = '', classes = [] } = {}, ...children) {
  const node = { tagName: tag.toUpperCase(), id, classList: classes, children, parentElement: null };
  for (const child of children) child.parentElement = node;
  return node;
}

const cta = el('button', { classes: ['cta'] });
const secondP = el('p');
const inHero = el('a');
const manyClasses = el('div', { classes: ['card', 'shadow', 'wide'] });
const escaped = el('div', { classes: ['md:flex'] });
const body = el('body', {},
  el('main', {}, cta, el('p'), secondP, manyClasses, escaped),
  el('section', { id: 'hero' }, inHero));
el('html', {}, body);

test('selectorFor', (t) => table(t, [
  ['body 아래부터 태그와 클래스로 경로를 만든다', cta, 'main > button.cta'],
  ['같은 태그의 형제가 있으면 순서로 구분한다', secondP, 'main > p:nth-of-type(2)'],
  ['조상에 id 가 있으면 id 부터 경로를 만든다', inHero, '#hero > a'],
  ['클래스는 앞의 두 개만 붙인다', manyClasses, 'main > div.card.shadow:nth-of-type(1)'],
  ['선택자에 쓸 수 없는 문자는 이스케이프한다', escaped, 'main > div.md\\:flex:nth-of-type(2)'],
  ['body 를 고르면 body 를 가리킨다', body, 'body'],
], (element, want) => assert.equal(selectorFor(element), want)));
