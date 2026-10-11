import { modeOf, openFile, pageFileName, reactionTo, watchList } from './view.mjs';
import { DEFAULT_SIZES, keyDelta, resize, togglePanel } from './layout.mjs';
import { frameBox, zoomBy, zoomForKey, zoomStep, zoomText } from './zoom.mjs';
import { elapsedText, parseSSE, stepText } from './chat.mjs';
import { createFrameTools } from './frame.mjs';
import { toolForKey } from './tools.mjs';
import { replaceText } from './textedit.mjs';

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => document.querySelectorAll(selector);
const frame = $('#frame');
const frameBoxElement = $('.frame-box');
const stage = $('.stage');
const openPath = $('#open-path');
const status = $('#status');
const promptForm = $('#prompt');
const promptInput = promptForm.elements.prompt;
const defaultPlaceholder = promptInput.placeholder;
const editor = CodeMirror($('#editor'), {
  theme: 'hunsu',
  lineNumbers: true,
  lineWrapping: true,
  extraKeys: { 'Cmd-S': () => save(), 'Ctrl-S': () => save() },
});
const frameTools = createFrameTools(frame, {
  onPick: showSelection,
  onKey: handleKey,
  onZoom: (deltaY) => setZoom(zoomBy(zoom, deltaY)),
  onEditText: (edit) => editText(edit),
  canEdit: () => !running,
});

let state = { open: null, preview: null };
let events;
let running = null;
let selection = null;
let panels = stored('hunsu.panels', { files: true, source: true });
let sizes = stored('hunsu.sizes', DEFAULT_SIZES);
let zoom = stored('hunsu.zoom', 1);
let deviceWidth = null;

function stored(key, fallback) {
  try {
    const value = JSON.parse(localStorage.getItem(key));
    if (value === null) return fallback;
    return typeof fallback === 'object' ? { ...fallback, ...value } : value;
  } catch {
    return fallback;
  }
}

function store(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {}
}

const fileURL = (path) => `/api/file?path=${encodeURIComponent(path)}`;

async function request(url, options) {
  const res = await fetch(url, options);
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
  status.textContent = '';
  return res;
}

const readFile = async (path) => (await request(fileURL(path))).text();

function fileButton(path, onClick, missing = false) {
  const button = document.createElement('button');
  button.textContent = path;
  button.classList.toggle('missing', missing);
  button.setAttribute('aria-current', path === state.open);
  button.onclick = onClick;
  const item = document.createElement('li');
  item.append(button);
  return item;
}

async function refreshList() {
  const { designSystem, pages, assets } = await (await request('/api/files')).json();
  $('#design-system').replaceChildren(...designSystem.map(({ path, exists }) =>
    fileButton(path, () => (exists ? open(path) : create(path)), !exists)));
  $('#pages').replaceChildren(...pages.map((path) => fileButton(path, () => open(path))));
  $('#assets').replaceChildren(...assets.map(assetLink));
  return { pages, files: [...designSystem.filter((e) => e.exists).map((e) => e.path), ...pages] };
}

function assetLink(path) {
  const link = document.createElement('a');
  link.href = `/p/${path.split('/').map(encodeURIComponent).join('/')}`;
  link.target = '_blank';
  link.textContent = path.slice(path.indexOf('/') + 1);
  const item = document.createElement('li');
  item.append(link);
  return item;
}

async function upload(files) {
  const results = await Promise.allSettled([...files].map((file) =>
    request(`/api/assets?name=${encodeURIComponent(file.name)}`, { method: 'POST', body: file })
      .catch((error) => { throw new Error(`${file.name}: ${error.message}`); })));
  await refreshList();
  if (state.preview) frame.contentWindow.location.reload();
  const failed = results.find((result) => result.status === 'rejected');
  if (failed) throw failed.reason;
}

const draggingFiles = (event) => event.dataTransfer?.types.includes('Files');

function acceptFileDrop(target) {
  target.addEventListener('dragover', (event) => {
    if (draggingFiles(event)) event.preventDefault();
  });
  target.addEventListener('drop', (event) => {
    if (!draggingFiles(event)) return;
    event.preventDefault();
    upload(event.dataTransfer.files);
  });
}

async function open(path) {
  if (!editor.isClean() && !confirm('저장하지 않은 변경을 버릴까요?')) return;
  const text = await readFile(path);
  const previousPreview = state.preview;
  state = openFile(state, path);
  editor.setOption('mode', modeOf(path));
  editor.setValue(text);
  editor.clearHistory();
  editor.markClean();
  showDirty();
  openPath.textContent = path;
  if (state.preview !== previousPreview) {
    frameTools.clear();
    frame.src = `/p/${encodeURIComponent(state.preview)}`;
  }
  watch();
  await refreshList();
}

async function loadModels() {
  const models = await (await request('/api/models')).json();
  const select = promptForm.elements.model;
  select.replaceChildren(...models.map((model, i) => new Option(i === 0 ? `${model.label} (기본)` : model.label, model.id)));
  select.hidden = models.length === 0;
}

function closeAll() {
  state = { open: null, preview: null };
  editor.setValue('');
  editor.markClean();
  openPath.textContent = '';
  frame.removeAttribute('src');
  watch();
  reportUI();
}

function reportUI() {
  fetch('/api/ui', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ page: state.preview ?? '', selection }),
  }).catch(() => {});
}

function listenCommands() {
  const commands = new EventSource('/api/ui/commands');
  commands.onopen = reportUI;
  commands.onmessage = ({ data }) => {
    const command = JSON.parse(data);
    if (command.kind === 'open') open(command.page);
  };
}

async function openExisting() {
  const { pages, files } = await refreshList();
  if (state.open && files.includes(state.open)) return;
  if (pages.length) await open(pages[0]);
  else closeAll();
}

async function create(path) {
  await request(fileURL(path), { method: 'POST' });
  await open(path);
}

async function save() {
  if (!state.open) return;
  await request(fileURL(state.open), { method: 'PUT', body: editor.getValue() });
  editor.markClean();
  showDirty();
}

function decodeHTML(raw) {
  const box = document.createElement('textarea');
  box.innerHTML = raw;
  return box.value;
}

async function editText(edit) {
  const page = state.preview;
  if (state.open === page && !editor.isClean()) await save();
  const next = replaceText(await readFile(page), edit, decodeHTML);
  if (next === null) {
    frame.contentWindow.location.reload();
    throw new Error('원본에서 이 문구를 찾지 못해 반영하지 않았습니다. 페이지 JS 가 만든 문구일 수 있습니다');
  }
  await request(fileURL(page), { method: 'PUT', body: next });
}

async function reloadEditor(path) {
  const text = await readFile(path);
  if (text === editor.getValue()) return;
  const cursor = editor.getCursor();
  editor.setValue(text);
  editor.setCursor(cursor);
  editor.markClean();
  showDirty();
}

function watch() {
  events?.close();
  const query = new URLSearchParams(watchList(state).map((path) => ['path', path]));
  events = new EventSource(`/api/events?${query}`);
  events.onmessage = ({ data: changed }) => {
    const reaction = reactionTo(changed, state, !editor.isClean());
    if (reaction.preview && state.preview) frame.contentWindow.location.reload();
    if (reaction.editor) reloadEditor(changed).catch(() => {});
  };
}

function showDirty() {
  openPath.dataset.dirty = !editor.isClean();
}

function addMessage(role, text) {
  const item = document.createElement('li');
  item.className = role;
  item.textContent = text;
  $('#messages').append(item);
  item.scrollIntoView({ block: 'end' });
  return item;
}

function addProgress() {
  const item = addMessage('progress running', '');
  const step = document.createElement('span');
  const elapsed = document.createElement('span');
  step.setAttribute('aria-live', 'polite');
  step.textContent = '요청 보내는 중';
  elapsed.className = 'elapsed';
  item.append(step, elapsed);
  const started = Date.now();
  const tick = () => { elapsed.textContent = elapsedText(Date.now() - started); };
  tick();
  const timer = setInterval(tick, 1000);
  return {
    show(text) { if (text) step.textContent = text; },
    finish(text) {
      clearInterval(timer);
      tick();
      item.classList.remove('running');
      this.show(text);
    },
  };
}

function setRunning(controller) {
  running = controller;
  const busy = Boolean(controller);
  editor.setOption('readOnly', busy);
  promptInput.disabled = busy;
  $('#undo').disabled = busy;
  $('#new-chat').disabled = busy;
  $('#send').textContent = busy ? '취소' : '보내기';
}

async function* streamEvents(res) {
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let rest = '';
  for (let chunk = await reader.read(); !chunk.done; chunk = await reader.read()) {
    const parsed = parseSSE(rest + chunk.value);
    rest = parsed.rest;
    yield* parsed.events;
  }
}

async function sendPrompt(prompt) {
  if (!editor.isClean()) await save();
  setRunning(new AbortController());
  addMessage('user', prompt);
  const progress = addProgress();
  let created = null;
  try {
    const res = await request('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt, page: state.preview ?? '', model: promptForm.elements.model.value, selection }),
      signal: running.signal,
    });
    for await (const event of streamEvents(res)) {
      if (event.kind === 'reply') addMessage('assistant', event.text);
      else if (event.kind === 'created') created ??= event.file;
      else progress.show(stepText(event));
    }
    progress.finish();
  } catch (error) {
    progress.finish(error.name === 'AbortError' ? '취소했습니다' : `오류: ${error.message}`);
  } finally {
    setRunning(null);
    if (created) await open(created);
    else await openExisting();
  }
}

function showSelection(next, picked) {
  selection = next;
  const chip = $('#selection');
  chip.hidden = !next;
  chip.querySelector('code').textContent = next?.selector ?? '';
  promptInput.placeholder = next ? '선택한 요소를 어떻게 바꿀까요?' : defaultPlaceholder;
  if (picked) promptInput.focus();
  reportUI();
}

function setTool(tool) {
  frameTools.setTool(tool);
  $$('[data-tool]').forEach((button) => button.setAttribute('aria-pressed', button.dataset.tool === tool));
}

function applyLayout() {
  document.body.classList.toggle('files-hidden', !panels.files);
  document.body.classList.toggle('source-hidden', !panels.source);
  for (const [kind, size] of Object.entries(sizes)) document.documentElement.style.setProperty(`--${kind}-size`, `${size}px`);
  $$('[data-panel]').forEach((button) => {
    const which = button.dataset.panel;
    button.setAttribute('aria-pressed', which === 'focus' ? !panels.files && !panels.source : panels[which]);
  });
  $$('[data-resize]').forEach((handle) => handle.setAttribute('aria-valuenow', sizes[handle.dataset.resize]));
  if (panels.source) editor.refresh();
  store('hunsu.panels', panels);
  store('hunsu.sizes', sizes);
}

function switchPanel(which) {
  panels = togglePanel(panels, which);
  applyLayout();
}

function layoutFrame() {
  const padding = deviceWidth ? 48 : 0;
  const box = frameBox({ width: deviceWidth, zoom, stage: { width: stage.clientWidth, height: stage.clientHeight - padding } });
  Object.assign(frame.style, { width: `${box.width}px`, height: `${box.height}px`, transform: `scale(${zoom})` });
  Object.assign(frameBoxElement.style, { width: `${box.boxWidth}px`, height: `${box.boxHeight}px` });
  $('#zoom-level').textContent = zoomText(zoom);
}

function setZoom(next) {
  zoom = next;
  layoutFrame();
  store('hunsu.zoom', zoom);
}

const isTyping = (target) => Boolean(target?.closest?.('input, textarea, select, [contenteditable], .CodeMirror'));

function handleKey(event) {
  if (event.metaKey || event.ctrlKey) {
    const nextZoom = zoomForKey(event.key, zoom);
    if (event.key === '\\') switchPanel('focus');
    else if (nextZoom !== null) setZoom(nextZoom);
    else return;
    event.preventDefault();
    return;
  }
  if (event.altKey || isTyping(event.target)) return;
  if (event.key === 'Escape') {
    frameTools.clear();
    return;
  }
  const tool = toolForKey(event.key);
  if (tool) {
    event.preventDefault();
    setTool(tool);
  }
}

function connectResizer(handle) {
  const kind = handle.dataset.resize;
  const axis = handle.getAttribute('aria-orientation') === 'vertical' ? 'x' : 'y';
  const coordinate = (event) => (axis === 'x' ? event.clientX : event.clientY);
  const layoutNow = () => ({ sizes, panels, viewport: { width: innerWidth, height: $('.source').clientHeight } });
  const update = (next) => {
    sizes = next;
    applyLayout();
  };
  handle.onpointerdown = (event) => {
    event.preventDefault();
    handle.setPointerCapture(event.pointerId);
    const origin = coordinate(event);
    const start = sizes[kind];
    document.body.classList.add('resizing', `resizing-${axis}`);
    handle.onpointermove = (move) => update(resize(layoutNow(), kind, start, coordinate(move) - origin));
  };
  handle.onlostpointercapture = () => {
    handle.onpointermove = null;
    document.body.classList.remove('resizing', `resizing-${axis}`);
  };
  handle.ondblclick = () => update({ ...sizes, [kind]: DEFAULT_SIZES[kind] });
  handle.onkeydown = (event) => {
    const delta = keyDelta(event.key, axis);
    if (!delta) return;
    event.preventDefault();
    update(resize(layoutNow(), kind, sizes[kind], delta));
  };
}

editor.on('change', showDirty);
$('#save').onclick = save;
$$('[data-width]').forEach((button) => {
  button.onclick = () => {
    deviceWidth = Number(button.dataset.width) || null;
    stage.classList.toggle('device', Boolean(deviceWidth));
    $$('[data-width]').forEach((other) => other.setAttribute('aria-pressed', other === button));
    layoutFrame();
  };
});
$$('[data-zoom]').forEach((button) => {
  button.onclick = () => setZoom(zoomStep(zoom, Number(button.dataset.zoom)));
});
$('#zoom-level').onclick = () => setZoom(1);
stage.addEventListener('wheel', (event) => {
  if (!event.ctrlKey) return;
  event.preventDefault();
  setZoom(zoomBy(zoom, event.deltaY));
}, { passive: false });
new ResizeObserver(layoutFrame).observe(stage);
$$('[data-tool]').forEach((button) => {
  button.onclick = () => setTool(button.dataset.tool);
});
$$('[data-panel]').forEach((button) => {
  button.onclick = () => switchPanel(button.dataset.panel);
});
$$('[data-resize]').forEach(connectResizer);
$('#selection button').onclick = () => frameTools.clear();
document.addEventListener('keydown', handleKey);
acceptFileDrop(document);
frame.addEventListener('load', () => acceptFileDrop(frame.contentDocument));
$('.upload input').onchange = async ({ target }) => {
  await upload(target.files);
  target.value = '';
};
$('#new-page').onsubmit = async (event) => {
  event.preventDefault();
  const input = event.target.elements.name;
  await create(pageFileName(input.value));
  input.value = '';
};
promptForm.onsubmit = (event) => {
  event.preventDefault();
  if (running) return running.abort();
  const prompt = promptInput.value.trim();
  if (!prompt) return;
  promptInput.value = '';
  sendPrompt(prompt);
};
promptInput.onkeydown = (event) => {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    promptForm.requestSubmit();
  }
};
$('#undo').onclick = async () => {
  await request('/api/undo', { method: 'POST' });
  addMessage('progress', '마지막 요청 전으로 되돌렸습니다');
  await openExisting();
};
$('#new-chat').onclick = async () => {
  await request('/api/chat/reset', { method: 'POST' });
  $('#messages').replaceChildren();
  addMessage('progress', '새 대화를 시작했습니다');
};
window.onunhandledrejection = ({ reason }) => { status.textContent = reason.message; };

applyLayout();
listenCommands();
await Promise.all([loadModels(), openExisting()]);
