const MAX_CLASSES = 2;

const escapeIdent = (name) => name.replace(/[^\w-]/g, '\\$&');

function step(node) {
  let part = node.tagName.toLowerCase();
  const classes = [...node.classList].slice(0, MAX_CLASSES);
  if (classes.length) part += `.${classes.map(escapeIdent).join('.')}`;
  const sameTag = [...node.parentElement.children].filter((sibling) => sibling.tagName === node.tagName);
  if (sameTag.length > 1) part += `:nth-of-type(${sameTag.indexOf(node) + 1})`;
  return part;
}

export function selectorFor(element) {
  const parts = [];
  for (let node = element; node.tagName !== 'BODY'; node = node.parentElement) {
    if (node.id) {
      parts.unshift(`#${escapeIdent(node.id)}`);
      break;
    }
    parts.unshift(step(node));
  }
  return parts.join(' > ') || 'body';
}
