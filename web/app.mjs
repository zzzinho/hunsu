import { modeOf, openFile, pageFileName, reactionTo, watchList } from './view.mjs';
import { DEFAULT_SIZES, keyDelta, resize, togglePanel } from './layout.mjs';
import { frameBox, zoomBy, zoomForKey, zoomStep, zoomText } from './zoom.mjs';
import { elapsedText, parseSSE, stepText, variantsText } from './chat.mjs';
import { createFrameTools } from './frame.mjs';
import { toolForKey } from './tools.mjs';
import { replaceText } from './textedit.mjs';
import { exportURL, filenameOf } from './download.mjs';
import { timeText, versionLabel } from './versions.mjs';
import { errorMessage, translatePage, translator } from './i18n.mjs';

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => document.querySelectorAll(selector);
const frame = $('#frame');
const frameBoxElement = $('.frame-box');
const stage = $('.stage');
const openPath = $('#open-path');
const status = $('#status');
const promptForm = $('#prompt');
const promptInput = promptForm.elements.prompt;
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
let t = translator(stored('hunsu.lang', 'en'));

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
  if (!res.ok) throw new Error(errorMessage(await res.text(), t));
  status.textContent = '';
  return res;
}

const readFile = async (path) => (await request(fileURL(path))).text();

function fileButton(path, onClick, missing = false) {
  const button = document.createElement('button');
  button.textContent = missing ? `${path} ${t('files.create')}` : path;
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
  if (!editor.isClean() && !confirm(t('confirm.discard'))) return;
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

const modelLabel = (model, isDefault) => [model.label, model.fast && t('model.fast'), isDefault && t('model.default')].filter(Boolean).join(' ');

async function checkStatus() {
  const status = await (await request('/api/status')).json();
  if (!status.ready) addMessage('notice', t(`status.${status.problem}`, { provider: status.provider, fix: status.fix }));
}

async function loadModels() {
  const models = await (await request('/api/models')).json();
  const select = promptForm.elements.model;
  select.replaceChildren(...models.map((model, i) => new Option(modelLabel(model, i === 0), model.id)));
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
    throw new Error(t('textEdit.notFound'));
  }
  await request(fileURL(page), { method: 'PUT', body: next });
}

async function download(kind) {
  $('#export').open = false;
  status.textContent = t('export.running');
  const res = await request(exportURL(kind, state.preview, deviceWidth));
  const link = document.createElement('a');
  link.href = URL.createObjectURL(await res.blob());
  link.download = filenameOf(res.headers.get('Content-Disposition') ?? '');
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}

function versionItem(version) {
  const item = document.createElement('li');
  const thumb = document.createElement(version.thumb ? 'img' : 'div');
  thumb.className = 'thumb';
  if (version.thumb) {
    thumb.src = `/api/versions/thumb?id=${version.id}`;
    thumb.alt = '';
    thumb.loading = 'lazy';
  } else {
    thumb.textContent = version.page || t('versions.noThumb');
  }
  const title = document.createElement('strong');
  title.textContent = `v${version.id} · ${versionLabel(version, t)}`;
  const meta = document.createElement('span');
  meta.className = 'meta';
  meta.textContent = [timeText(new Date(version.time), new Date(), t), version.page].filter(Boolean).join(' · ');
  const restore = document.createElement('button');
  restore.type = 'button';
  restore.textContent = t('versions.restore');
  restore.disabled = Boolean(running);
  restore.onclick = () => noteErrors(restoreVersion(version.id));
  item.append(thumb, title, meta, restore);
  return item;
}

const noteErrors = (promise) => promise.catch((error) => { $('#version-note').textContent = error.message; });

async function showVersions() {
  $('#version-note').textContent = '';
  const versions = await (await request('/api/versions')).json();
  $('#version-list').replaceChildren(...versions.map(versionItem));
  $('#version-save').disabled = Boolean(running);
}

async function restoreVersion(id) {
  if (!editor.isClean()) await save();
  await request(`/api/versions/restore?id=${id}&page=${encodeURIComponent(state.preview ?? '')}`, { method: 'POST' });
  addMessage('progress', t('versions.restored', { id }));
  await Promise.all([showVersions(), openExisting()]);
  $('#version-note').textContent = t('versions.restored', { id });
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
  step.textContent = t('step.sending');
  elapsed.className = 'elapsed';
  item.append(step, elapsed);
  const started = Date.now();
  const tick = () => { elapsed.textContent = elapsedText(Date.now() - started, t); };
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
  promptForm.elements.variants.disabled = busy;
  $('#undo').disabled = busy;
  $('#new-chat').disabled = busy;
  $('#send').textContent = busy ? t('cancel') : t('send');
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
  const count = Number(promptForm.elements.variants.value);
  setRunning(new AbortController());
  addMessage('user', count > 1 ? t('variants.request', { prompt, count }) : prompt);
  const progress = addProgress();
  const steps = {};
  let created = null;
  let ended = false;
  try {
    const res = await request(count > 1 ? '/api/chat/variants' : '/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt, page: state.preview ?? '', model: promptForm.elements.model.value, selection, count }),
      signal: running.signal,
    });
    for await (const event of streamEvents(res)) {
      if (event.variant) {
        steps[event.variant] = stepText(event, t) ?? steps[event.variant];
        progress.show(variantsText(steps, t));
      } else if (event.kind === 'reply') addMessage('assistant', event.text);
      else if (event.kind === 'created') created ??= event.file;
      else {
        progress.show(stepText(event, t));
        ended ||= event.kind === 'end';
      }
    }
    progress.finish();
  } catch (error) {
    progress.finish(error.name === 'AbortError' ? t('message.canceled') : t('message.error', { message: error.message }));
  } finally {
    setRunning(null);
    if (created) await open(created);
    else await openExisting();
  }
  if (count > 1 && ended) {
    const item = addMessage('progress', t('variants.made', { count }));
    const reopen = document.createElement('button');
    reopen.type = 'button';
    reopen.textContent = t('variants.show');
    reopen.onclick = () => showVariants();
    item.append(reopen);
    await showVariants();
  }
}

function variantColumn(variant) {
  const item = document.createElement('li');
  const title = document.createElement('strong');
  title.textContent = t('variants.take', { id: variant.id });
  const reply = document.createElement('p');
  reply.className = variant.error ? 'reply error' : 'reply';
  reply.textContent = variant.error || variant.reply;
  item.append(title, reply);
  if (!variant.error) {
    const shot = document.createElement('div');
    shot.className = 'shot';
    const preview = document.createElement('iframe');
    preview.title = t('variants.preview', { id: variant.id });
    preview.src = `/variant/${variant.id}/${encodeURIComponent(variant.page)}`;
    shot.append(preview);
    item.append(shot);
  }
  const choose = document.createElement('button');
  choose.type = 'button';
  choose.className = 'primary';
  choose.textContent = t('variants.pick');
  choose.disabled = Boolean(variant.error);
  choose.onclick = () => noteVariantErrors(chooseVariant(variant));
  item.append(choose);
  return item;
}

const noteVariantErrors = (promise) => promise.catch((error) => { $('#variant-note').textContent = error.message; });

// 미리보기는 보고 있는 너비로 렌더하고, 칸 너비에 맞게 줄여 보인다.
function fitVariantFrames() {
  const width = deviceWidth ?? 1280;
  $$('#variant-list iframe').forEach((preview) => {
    const shot = preview.parentElement;
    const zoom = shot.clientWidth / width;
    Object.assign(preview.style, { width: `${width}px`, height: `${shot.clientHeight / zoom}px`, zoom });
  });
}

async function showVariants() {
  const variants = await (await request('/api/variants')).json();
  if (!variants.length) {
    status.textContent = t('variants.none');
    return;
  }
  $('#variant-note').textContent = '';
  $('#variant-list').replaceChildren(...variants.map(variantColumn));
  $('#variants-dialog').showModal();
  fitVariantFrames();
}

async function chooseVariant(variant) {
  if (!editor.isClean()) await save();
  await request(`/api/variants/choose?id=${variant.id}&page=${encodeURIComponent(state.preview ?? '')}`, { method: 'POST' });
  $('#variants-dialog').close();
  addMessage('progress', t('variants.picked', { id: variant.id }));
  await open(variant.page);
}

function showSelection(next, picked) {
  selection = next;
  const chip = $('#selection');
  chip.hidden = !next;
  chip.querySelector('code').textContent = next?.selector ?? '';
  showPlaceholder();
  if (picked) promptInput.focus();
  reportUI();
}

const showPlaceholder = () => {
  promptInput.placeholder = t(selection ? 'prompt.placeholderSelected' : 'prompt.placeholder');
};

function applyLanguage() {
  document.documentElement.lang = t.lang;
  translatePage(document, t);
  $('#lang').value = t.lang;
  showPlaceholder();
  $('#send').textContent = running ? t('cancel') : t('send');
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
$('#export').ontoggle = () => {
  $$('[data-export][data-page]').forEach((button) => { button.disabled = !state.preview; });
};
$$('[data-export]').forEach((button) => {
  button.onclick = () => download(button.dataset.export);
});
document.addEventListener('click', (event) => {
  if (!event.target.closest('#export')) $('#export').open = false;
});
$('#variants-close').onclick = () => $('#variants-dialog').close();
$('#variants-discard').onclick = () => noteVariantErrors((async () => {
  await request('/api/variants/discard', { method: 'POST' });
  $('#variants-dialog').close();
  addMessage('progress', t('variants.discarded'));
})());
$('#versions-open').onclick = async () => {
  await showVersions();
  $('#versions').showModal();
};
$('#versions-close').onclick = () => $('#versions').close();
$('#version-save').onclick = () => noteErrors((async () => {
  if (!editor.isClean()) await save();
  const res = await request(`/api/versions?page=${encodeURIComponent(state.preview ?? '')}`, { method: 'POST' });
  const { saved } = await res.json();
  await showVersions();
  $('#version-note').textContent = saved ? t('versions.saved') : t('versions.same');
})());
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
  addMessage('progress', t('message.undone'));
  await openExisting();
};
$('#new-chat').onclick = async () => {
  await request('/api/chat/reset', { method: 'POST' });
  $('#messages').replaceChildren();
  addMessage('progress', t('message.newChat'));
};
$('#lang').onchange = async ({ target }) => {
  t = translator(target.value);
  store('hunsu.lang', t.lang);
  applyLanguage();
  await Promise.all([loadModels(), refreshList()]);
};
window.onunhandledrejection = ({ reason }) => { status.textContent = reason.message; };

applyLanguage();
applyLayout();
listenCommands();
await Promise.all([loadModels(), openExisting(), checkStatus()]);
