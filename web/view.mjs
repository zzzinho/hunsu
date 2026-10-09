const DESIGN_CSS = 'design.css';

const modes = { html: 'htmlmixed', css: 'css', md: 'markdown' };

const isPage = (path) => path.endsWith('.html');

export const modeOf = (path) => modes[path.split('.').pop()] ?? null;

export const pageFileName = (name) => {
  const trimmed = name.trim();
  return isPage(trimmed) ? trimmed : `${trimmed}.html`;
};

export const openFile = (state, path) => ({ open: path, preview: isPage(path) ? path : state.preview });

export const watchList = ({ open, preview }) => [...new Set([open, preview, DESIGN_CSS].filter(Boolean))];

export const reactionTo = (changed, state, dirty) => ({
  preview: changed === state.preview || changed === DESIGN_CSS,
  editor: changed === state.open && !dirty,
});
