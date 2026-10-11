import { test } from 'node:test';
import assert from 'node:assert/strict';
import { dictionaries, errorMessage, translator } from './i18n.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

test('translator', (t) => table(t, [
  ['고른 언어의 문구를 돌려준다', 'ko', 'send', {}, '보내기'],
  ['영어가 기본이다', 'en', 'send', {}, 'Send'],
  ['없는 언어를 고르면 영어를 쓴다', 'fr', 'send', {}, 'Send'],
  ['자리표시를 값으로 바꾼다', 'en', 'step.write', { file: 'a.html' }, 'Writing a.html'],
  ['사전에 없는 키는 키를 그대로 보인다', 'ko', 'no.such.key', {}, 'no.such.key'],
], (lang, key, params, want) => assert.equal(translator(lang)(key, params), want)));

test('errorMessage', (t) => table(t, [
  ['서버 오류 코드를 고른 언어로 바꾼다', '{"code":"busy","message":"already handling a request"}', '이미 요청을 처리하고 있습니다'],
  ['번역이 없는 코드는 서버 메시지를 그대로 보인다', '{"code":"internal","message":"disk full"}', 'disk full'],
  ['JSON 이 아닌 본문은 그대로 보인다', 'Method Not Allowed', 'Method Not Allowed'],
], (body, want) => assert.equal(errorMessage(body, translator('ko')), want)));

test('dictionaries', (t) => table(t, [
  ['한국어 사전은 영어 사전과 같은 키를 가진다', Object.keys(dictionaries.ko).sort(), Object.keys(dictionaries.en).sort()],
], (got, want) => assert.deepEqual(got, want)));
