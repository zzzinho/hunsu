import { test } from 'node:test';
import assert from 'node:assert/strict';
import { exportURL, filenameOf } from './download.mjs';

async function table(t, rows, check) {
  for (const [name, ...args] of rows) await t.test(name, () => check(...args));
}

test('exportURL', (t) => table(t, [
  ['ZIP 은 열린 페이지와 관계없이 폴더 전체를 받는다', 'zip', null, null, '/api/export/zip'],
  ['HTML 은 보고 있는 페이지를 받는다', 'html', '메뉴 a.html', null, '/api/export/html?page=%EB%A9%94%EB%89%B4%20a.html'],
  ['PNG 는 모바일 너비로 보고 있으면 그 너비로 찍는다', 'png', 'a.html', 390, '/api/export/png?page=a.html&width=390'],
  ['PNG 는 데스크톱으로 보고 있으면 서버의 기본 너비로 찍는다', 'png', 'a.html', null, '/api/export/png?page=a.html'],
], (kind, page, width, want) => assert.equal(exportURL(kind, page, width), want)));

test('filenameOf', (t) => table(t, [
  ['Content-Disposition 의 파일 이름을 쓴다', 'attachment; filename=a.html', 'a.html'],
  ['따옴표로 감싼 파일 이름도 읽는다', 'attachment; filename="my page.png"', 'my page.png'],
  ['UTF-8 로 인코딩된 파일 이름을 풀어 쓴다', "attachment; filename*=utf-8''%EB%94%94%EC%9E%90%EC%9D%B8.zip", '디자인.zip'],
  ['파일 이름이 없으면 download 로 받는다', '', 'download'],
], (header, want) => assert.equal(filenameOf(header), want)));
