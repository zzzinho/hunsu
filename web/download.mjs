export function exportURL(kind, page, width) {
  if (kind === 'zip') return '/api/export/zip';
  const url = `/api/export/${kind}?page=${encodeURIComponent(page)}`;
  return kind === 'png' && width ? `${url}&width=${width}` : url;
}

export function filenameOf(disposition) {
  const encoded = /filename\*=utf-8''([^;]+)/i.exec(disposition);
  if (encoded) return decodeURIComponent(encoded[1]);
  return /filename="?([^";]+)"?/i.exec(disposition)?.[1] ?? 'download';
}
