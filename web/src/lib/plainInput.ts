/**
 * Text fields in Mesa hold identifiers, credentials, SQL and cell values, so
 * none of them should be auto-capitalized, autocorrected or spell-checked.
 * WebKit (Safari and the macOS desktop app) capitalizes the first letter by
 * default, which turned "root" into "Root". A field that holds prose opts
 * back in with data-prose.
 */
const FIELDS = "input, textarea";

function plain(el: Element) {
  if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement)) return;
  if (el.dataset.prose !== undefined) return;
  if (!el.hasAttribute("autocapitalize")) el.setAttribute("autocapitalize", "off");
  if (!el.hasAttribute("autocorrect")) el.setAttribute("autocorrect", "off");
  if (!el.hasAttribute("spellcheck")) el.setAttribute("spellcheck", "false");
}

function plainWithin(node: Node) {
  if (!(node instanceof Element)) return;
  plain(node);
  node.querySelectorAll(FIELDS).forEach(plain);
}

/** Applies the rule to every field under root, now and as fields are added. */
export function plainTextFields(root: HTMLElement): () => void {
  plainWithin(root);
  const observer = new MutationObserver((records) => {
    for (const r of records) r.addedNodes.forEach(plainWithin);
  });
  observer.observe(root, { childList: true, subtree: true });
  return () => observer.disconnect();
}
