// 렌더 화면에 문구로 보이지 않는 요소. 원본과 렌더 문서에서 같은 기준으로 문구를 센다.
export const HIDDEN_TEXT = ['script', 'style', 'title', 'textarea', 'noscript', 'template'];

const TAG_START = /<(\/?)([a-zA-Z][^\s/>]*)|<[!?]/y;

const escapes = { '&': '&amp;', '<': '&lt;', '>': '&gt;', ' ': '&nbsp;' };
const escapeText = (text) => text.replace(/[&<> ]/g, (char) => escapes[char]);

function tagEnd(source, from) {
  let quote = null;
  for (let i = from; i < source.length; i++) {
    const char = source[i];
    if (quote) {
      if (char === quote) quote = null;
    } else if (char === '"' || char === "'") {
      quote = char;
    } else if (char === '>') {
      return i + 1;
    }
  }
  return source.length;
}

function skipTo(source, pattern, from) {
  pattern.lastIndex = from;
  const found = pattern.exec(source);
  return found ? found.index : source.length;
}

function textSegments(source) {
  const segments = [];
  let start = 0;
  let i = 0;
  const flush = (end) => {
    if (end > start) segments.push({ start, end });
  };
  while (i < source.length) {
    if (source[i] !== '<') {
      i++;
      continue;
    }
    if (source.startsWith('<!--', i)) {
      flush(i);
      const close = source.indexOf('-->', i + 4);
      i = start = close < 0 ? source.length : close + 3;
      continue;
    }
    TAG_START.lastIndex = i;
    const tag = TAG_START.exec(source);
    if (!tag) {
      i++;
      continue;
    }
    flush(i);
    i = start = tagEnd(source, i + 1);
    const name = tag[2]?.toLowerCase();
    if (!tag[1] && HIDDEN_TEXT.includes(name)) i = start = skipTo(source, new RegExp(`</${name}[\\s/>]`, 'gi'), i);
  }
  flush(source.length);
  return segments;
}

// 렌더 문서에서 같은 문구 중 occurrence 번째인 문구를 원본에서 찾아 바꾼다. 찾지 못하면 null.
export function replaceText(source, { text, occurrence, value }, decode) {
  const key = text.trim();
  if (!key) return null;
  const target = textSegments(source)
    .filter(({ start, end }) => decode(source.slice(start, end)).trim() === key)[occurrence];
  if (!target) return null;
  const raw = source.slice(target.start, target.end);
  const lead = raw.match(/^\s*/)[0];
  const trail = raw.slice(lead.length).match(/\s*$/)[0];
  return source.slice(0, target.start) + lead + escapeText(value.trim()) + trail + source.slice(target.end);
}
