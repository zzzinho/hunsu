import { test } from 'node:test';
import assert from 'node:assert/strict';
import { keyDelta, resize, togglePanel } from './layout.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const allOpen = { files: true, source: true };

test('togglePanel', (t) => table(t, [
  ['파일 패널을 접는다', allOpen, 'files', { files: false, source: true }],
  ['접힌 편집기·채팅 패널을 편다', { files: true, source: false }, 'source', allOpen],
  ['렌더만 보기는 열린 패널을 모두 접는다', { files: true, source: false }, 'focus', { files: false, source: false }],
  ['모두 접힌 상태에서 렌더만 보기를 끄면 모두 편다', { files: false, source: false }, 'focus', allOpen],
], (panels, which, want) => assert.deepEqual(togglePanel(panels, which), want)));

const roomy = { sizes: { files: 220, source: 500, chat: 300 }, panels: allOpen, viewport: { width: 1400, height: 800 } };
const filesHidden = { ...roomy, panels: { files: false, source: true } };

test('resize', (t) => table(t, [
  ['파일 패널은 경계를 오른쪽으로 끌면 넓어진다', roomy, 'files', 220, 50, 270],
  ['편집기·채팅 패널은 경계를 왼쪽으로 끌면 넓어진다', roomy, 'source', 500, -60, 560],
  ['채팅은 경계를 위로 끌면 높아진다', roomy, 'chat', 300, -40, 340],
  ['최소 크기보다 작아지지 않는다', roomy, 'files', 220, -200, 160],
  ['최대 크기보다 커지지 않는다', roomy, 'files', 220, 500, 480],
  ['렌더 영역이 320px 은 남도록 커진다', roomy, 'source', 500, -900, 860],
  ['접힌 패널의 자리는 렌더 영역에 돌려준 것으로 본다', filesHidden, 'source', 500, -900, 900],
  ['채팅은 편집기가 120px 은 남도록 커진다', roomy, 'chat', 300, -900, 640],
], (layout, kind, start, delta, want) => assert.equal(resize(layout, kind, start, delta)[kind], want)));

test('keyDelta', (t) => table(t, [
  ['세로 경계는 오른쪽 화살표로 16px 오른쪽으로 옮긴다', 'ArrowRight', 'x', 16],
  ['가로 경계는 위 화살표로 16px 위로 옮긴다', 'ArrowUp', 'y', -16],
  ['경계 방향과 맞지 않는 화살표는 무시한다', 'ArrowUp', 'x', 0],
], (key, axis, want) => assert.equal(keyDelta(key, axis), want)));
