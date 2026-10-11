const DESIGN_CSS = 'design.css';

const modes = { html: 'htmlmixed', css: 'css', md: 'markdown' };

const isPage = (path) => path.endsWith('.html');

export const modeOf = (path) => modes[path.split('.').pop()] ?? null;

export const pageFileName = (name) => {
  const trimmed = name.trim();
  return isPage(trimmed) ? trimmed : `${trimmed}.html`;
};

// 렌더 iframe 안에서 링크로 이동했을 때 어느 페이지인지 알아낸다.
export function pageFromPath(pathname) {
  const match = /^\/p\/([^/]+)$/.exec(pathname);
  if (!match) return null;
  try {
    const name = decodeURIComponent(match[1]);
    return isPage(name) ? name : null;
  } catch {
    return null;
  }
}

export const openFile = (state, path) => ({ open: path, preview: isPage(path) ? path : state.preview });

export const watchList = ({ open, preview }) => [...new Set([open, preview, DESIGN_CSS].filter(Boolean))];

export const reactionTo = (changed, state, dirty) => ({
  preview: changed === state.preview || changed === DESIGN_CSS,
  editor: changed === state.open && !dirty,
});
