import { test } from 'node:test';
import assert from 'node:assert/strict';
import { timeText, versionLabel } from './versions.mjs';
import { translator } from './i18n.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

const now = new Date(2026, 9, 12, 18, 0);

test('timeText', (t) => table(t, [
  ['오늘 남긴 버전은 시각만 보인다', 'en', new Date(2026, 9, 12, 9, 5), 'Today 09:05'],
  ['다른 날 남긴 버전은 그 언어의 날짜와 시각이 보인다', 'en', new Date(2026, 9, 3, 14, 30), 'Oct 3 14:30'],
  ['한국어로는 한국어 날짜로 보인다', 'ko', new Date(2026, 9, 3, 14, 30), '10월 3일 14:30'],
], (lang, date, want) => assert.equal(timeText(date, now, translator(lang)), want)));

const en = translator('en');

test('versionLabel', (t) => table(t, [
  ['요청 버전은 요청 내용을 이름으로 쓴다', { kind: 'request', text: '버튼 둥글게' }, '버튼 둥글게'],
  ['요청 전 상태', { kind: 'before-request' }, 'Before request'],
  ['직접 저장한 버전', { kind: 'saved' }, 'Saved by you'],
  ['복원 전 상태', { kind: 'before-restore' }, 'Before restore'],
  ['복원한 버전은 복원한 번호와 함께', { kind: 'restore', ref: 3 }, 'Restored v3'],
  ['고르지 않은 비교안은 안 번호와 요청 내용으로', { kind: 'variant', ref: 1, text: '둥글게' }, 'Take 1 · 둥글게'],
  ['고른 비교안은 고른 안 번호와 요청 내용으로', { kind: 'chosen', ref: 2, text: '둥글게' }, 'Picked take 2 · 둥글게'],
  ['모르는 종류는 그 이름을 그대로 보인다', { kind: 'future-kind' }, 'future-kind'],
], (version, want) => assert.equal(versionLabel(version, en), want)));
