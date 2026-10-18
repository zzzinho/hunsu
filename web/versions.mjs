const pad = (n) => String(n).padStart(2, '0');

export function timeText(date, now, t) {
  const clock = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  if (date.toDateString() === now.toDateString()) return t('time.today', { clock });
  return `${new Intl.DateTimeFormat(t.lang, { month: 'short', day: 'numeric' }).format(date)} ${clock}`;
}

const kindKeys = {
  'before-request': 'version.beforeRequest',
  saved: 'version.saved',
  'before-restore': 'version.beforeRestore',
  'before-undo': 'version.beforeUndo',
  restore: 'version.restore',
  variant: 'version.variant',
  chosen: 'version.chosen',
};

export function versionLabel(version, t) {
  if (version.kind === 'request') return version.text;
  const key = kindKeys[version.kind];
  return key ? t(key, { ref: version.ref, text: version.text }) : version.kind;
}
