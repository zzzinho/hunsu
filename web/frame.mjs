import { selectorFor } from './selector.mjs';
import { isDrag, panPosition } from './tools.mjs';
import { HIDDEN_TEXT } from './textedit.mjs';

// 디자인 파일이 아니라 렌더된 문서에만 넣는다. 페이지 CSS 가 덮어쓰지 못하게 id 선택자로 지정한다.
const OVERLAY_CSS = `
#hunsu-hover, #hunsu-selected { position: fixed; display: none; pointer-events: none; z-index: 2147483647; box-sizing: border-box; border-radius: 2px; }
#hunsu-hover { border: 1px solid #e0a64b; background: rgb(224 166 75 / 10%); }
#hunsu-selected { border: 2px solid #e0a64b; }
[data-hunsu-editing] { outline: 2px solid #e0a64b !important; outline-offset: 2px; }
[data-hunsu-editing], [data-hunsu-editing] * { cursor: text !important; user-select: text !important; }
`;

const CURSOR_CSS = {
  select: '*, *::before, *::after { cursor: default !important; }',
  hand: '*, *::before, *::after { cursor: grab !important; user-select: none !important; }',
  panning: '*, *::before, *::after { cursor: grabbing !important; user-select: none !important; }',
};

export function createFrameTools(frame, { onPick, onKey, onZoom, onEditText, canEdit }) {
  let tool = 'select';
  let selector = null;
  let pan = null;
  let suppressClick = false;
  let editing = null;

  const doc = () => frame.contentDocument;
  const win = () => frame.contentWindow;

  function selectedElement() {
    try {
      return selector ? doc()?.querySelector(selector) : null;
    } catch {
      return null;
    }
  }

  function place(id, element) {
    const box = doc()?.getElementById(id);
    if (!box) return;
    if (!element) {
      box.style.display = 'none';
      return;
    }
    const rect = element.getBoundingClientRect();
    Object.assign(box.style, {
      display: 'block', left: `${rect.left}px`, top: `${rect.top}px`, width: `${rect.width}px`, height: `${rect.height}px`,
    });
  }

  function applyTool() {
    const style = doc()?.getElementById('hunsu-style');
    if (style) style.textContent = OVERLAY_CSS + CURSOR_CSS[pan?.moved ? 'panning' : tool];
    if (tool !== 'select') place('hunsu-hover', null);
  }

  function select(element) {
    selector = selectorFor(element);
    place('hunsu-selected', element);
    onPick({ selector, html: element.outerHTML }, true);
  }

  function editableText(element) {
    const [node, ...rest] = element.childNodes;
    return node?.nodeType === Node.TEXT_NODE && !rest.length && node.data.trim() ? node : null;
  }

  function occurrenceOf(node) {
    const key = node.data.trim();
    const walker = doc().createTreeWalker(doc().documentElement, NodeFilter.SHOW_TEXT);
    let count = 0;
    for (let other = walker.nextNode(); other && other !== node; other = walker.nextNode()) {
      if (other.data.trim() === key && !other.parentElement?.closest(HIDDEN_TEXT.join(','))) count++;
    }
    return count;
  }

  function onEditKey(event) {
    if (event.key === 'Enter' && !event.isComposing) {
      event.preventDefault();
      finishEdit(true);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      finishEdit(false);
    }
  }

  function startEdit(element) {
    const node = editableText(element);
    if (!node || !canEdit()) return;
    editing = { element, text: node.data, occurrence: occurrenceOf(node) };
    element.setAttribute('contenteditable', 'plaintext-only');
    element.dataset.hunsuEditing = '';
    element.addEventListener('keydown', onEditKey);
    element.addEventListener('blur', () => finishEdit(true), { once: true });
    element.focus();
    win().getSelection().selectAllChildren(element);
  }

  function finishEdit(commit) {
    if (!editing) return;
    const { element, text, occurrence } = editing;
    editing = null;
    element.removeEventListener('keydown', onEditKey);
    element.removeAttribute('contenteditable');
    delete element.dataset.hunsuEditing;
    const value = element.textContent;
    if (!commit) element.textContent = text;
    else if (value.trim() !== text.trim()) onEditText({ text, occurrence, value });
  }

  function endPan() {
    suppressClick = pan.moved;
    pan = null;
    applyTool();
  }

  function onMove(event) {
    if (pan && !(event.buttons & 1)) endPan();
    if (pan) {
      const point = { x: event.clientX, y: event.clientY };
      pan.moved ||= isDrag(pan.start, point);
      if (!pan.moved) return;
      const { left, top } = panPosition(pan.start, point);
      win().scrollTo(left, top);
      applyTool();
      return;
    }
    if (tool === 'select') place('hunsu-hover', event.target);
  }

  function onDown(event) {
    if (event.button !== 0) return;
    if (editing?.element.contains(event.target)) return;
    finishEdit(true);
    if (tool === 'select') {
      event.preventDefault();
      return;
    }
    event.preventDefault();
    pan = { start: { x: event.clientX, y: event.clientY, scrollX: win().scrollX, scrollY: win().scrollY }, moved: false };
  }

  function onClick(event) {
    if (editing?.element.contains(event.target)) return;
    if (tool === 'select') {
      event.preventDefault();
      event.stopPropagation();
      select(event.target);
    } else if (suppressClick) {
      event.preventDefault();
      event.stopPropagation();
    }
    suppressClick = false;
  }

  function reposition() {
    place('hunsu-hover', null);
    place('hunsu-selected', selectedElement());
  }

  function attach() {
    const d = doc();
    editing = null;
    if (!d?.documentElement) return;
    const style = d.createElement('style');
    style.id = 'hunsu-style';
    const hover = d.createElement('div');
    hover.id = 'hunsu-hover';
    const selected = d.createElement('div');
    selected.id = 'hunsu-selected';
    d.documentElement.append(style, hover, selected);
    applyTool();

    const element = selectedElement();
    if (element) {
      place('hunsu-selected', element);
      onPick({ selector, html: element.outerHTML }, false);
    } else if (selector) {
      selector = null;
      onPick(null, false);
    }

    d.addEventListener('mousemove', onMove, true);
    d.addEventListener('mousedown', onDown, true);
    d.addEventListener('mouseup', () => pan && endPan(), true);
    d.addEventListener('click', onClick, true);
    d.addEventListener('dblclick', (event) => {
      if (tool === 'select' && !editing) startEdit(event.target);
    }, true);
    d.addEventListener('keydown', onKey, true);
    d.addEventListener('wheel', (event) => {
      if (!event.ctrlKey) return;
      event.preventDefault();
      onZoom(event.deltaY);
    }, { capture: true, passive: false });
    win().addEventListener('scroll', reposition, true);
    win().addEventListener('resize', reposition);
  }

  frame.addEventListener('load', attach);

  return {
    setTool(next) {
      finishEdit(true);
      tool = next;
      pan = null;
      applyTool();
    },
    clear() {
      selector = null;
      place('hunsu-selected', null);
      onPick(null, false);
    },
  };
}
