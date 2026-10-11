import { test } from 'node:test';
import assert from 'node:assert/strict';
import { timeText } from './versions.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const now = new Date(2026, 9, 12, 18, 0);

test('timeText', (t) => table(t, [
  ['오늘 남긴 버전은 시각만 보인다', new Date(2026, 9, 12, 9, 5), '오늘 09:05'],
  ['다른 날 남긴 버전은 날짜와 시각이 보인다', new Date(2026, 9, 3, 14, 30), '10월 3일 14:30'],
], (date, want) => assert.equal(timeText(date, now), want)));
