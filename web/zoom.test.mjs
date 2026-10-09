import { test } from 'node:test';
import assert from 'node:assert/strict';
import { frameBox, zoomBy, zoomForKey, zoomStep, zoomText } from './zoom.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

test('zoomStep', (t) => table(t, [
  ['확대하면 다음 단계로 커진다', 1, 1, 1.25],
  ['축소하면 이전 단계로 작아진다', 1, -1, 0.75],
  ['단계 사이 값에서 확대하면 바로 위 단계로 간다', 0.9, 1, 1],
  ['가장 큰 단계보다 커지지 않는다', 3, 1, 3],
  ['가장 작은 단계보다 작아지지 않는다', 0.25, -1, 0.25],
], (zoom, direction, want) => assert.equal(zoomStep(zoom, direction), want)));

test('zoomBy', (t) => table(t, [
  ['핀치로 벌리면 조금씩 커진다', 1, -10, 1.11],
  ['핀치로 오므리면 조금씩 작아진다', 1, 10, 0.9],
  ['핀치로도 가장 작은 배율보다 작아지지 않는다', 0.3, 500, 0.25],
], (zoom, deltaY, want) => assert.equal(zoomBy(zoom, deltaY), want)));

test('zoomForKey', (t) => table(t, [
  ['⌘= 는 확대한다', '=', 1, 1.25],
  ['⌘+ 도 확대한다', '+', 1, 1.25],
  ['⌘- 는 축소한다', '-', 1, 0.75],
  ['⌘0 은 100% 로 돌아간다', '0', 2, 1],
  ['그 밖의 키는 배율을 바꾸지 않는다', 'a', 2, null],
], (key, zoom, want) => assert.equal(zoomForKey(key, zoom), want)));

const stage = { width: 800, height: 600 };

test('frameBox', (t) => table(t, [
  ['데스크톱을 축소하면 더 넓은 화면으로 렌더해 작게 보인다', { width: null, zoom: 0.5, stage }, { width: 1600, height: 1200, boxWidth: 800, boxHeight: 600 }],
  ['모바일은 너비 390px 을 유지하고 배율만큼 크게 보인다', { width: 390, zoom: 2, stage }, { width: 390, height: 300, boxWidth: 780, boxHeight: 600 }],
], (input, want) => assert.deepEqual(frameBox(input), want)));

test('zoomText', (t) => table(t, [
  ['배율을 퍼센트로 보인다', 1.25, '125%'],
  ['소수점 아래는 반올림한다', 1.105, '111%'],
], (zoom, want) => assert.equal(zoomText(zoom), want)));
