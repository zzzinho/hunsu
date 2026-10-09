import { test } from 'node:test';
import assert from 'node:assert/strict';
import { isDrag, panPosition, toolForKey } from './tools.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const start = { x: 100, y: 100, scrollX: 0, scrollY: 500 };

test('toolForKey', (t) => table(t, [
  ['V 는 선택 도구다', 'v', 'select'],
  ['H 는 손 도구다', 'H', 'hand'],
  ['그 밖의 키는 도구를 바꾸지 않는다', 'a', null],
], (key, want) => assert.equal(toolForKey(key), want)));

test('isDrag', (t) => table(t, [
  ['3px 이하로 움직이면 클릭으로 본다', { x: 102, y: 101 }, false],
  ['3px 넘게 움직이면 끌기로 본다', { x: 100, y: 105 }, true],
], (point, want) => assert.equal(isDrag(start, point), want)));

test('panPosition', (t) => table(t, [
  ['아래로 끌면 페이지 위쪽이 보이도록 올라간다', { x: 100, y: 160 }, { left: 0, top: 440 }],
  ['오른쪽으로 끌면 페이지 왼쪽이 보이도록 이동한다', { x: 130, y: 100 }, { left: -30, top: 500 }],
], (point, want) => assert.deepEqual(panPosition(start, point), want)));
