export const EXTERNAL_LINK_ROUTE_NAME = 'external-link';

export interface ExternalWarningRoute {
  name: typeof EXTERNAL_LINK_ROUTE_NAME;
  query: { url: string };
}

interface ExternalLinkRouter {
  push(to: ExternalWarningRoute): unknown;
  resolve(to: ExternalWarningRoute): { href: string };
}

export interface OpenExternalLinkOptions {
  newTab?: boolean;
  currentOrigin?: string;
  trustedDomains?: readonly string[];
}

export type ExternalLinkNavigator = (url: string) => unknown;

function browserOrigin() {
  return typeof window === 'undefined' ? '' : window.location.origin;
}

/** Return a canonical absolute HTTP(S) URL, or an empty string. */
export function normalizeHttpUrl(raw: string) {
  const value = raw.trim();
  if (!value) return '';
  try {
    const url = new URL(value);
    return ['http:', 'https:'].includes(url.protocol) ? url.href : '';
  } catch {
    return '';
  }
}

export function normalizeTrustedDomain(raw: string) {
  let domain = raw.trim().toLowerCase();
  domain = domain.replace(/^\*\./, '').replace(/^\./, '').replace(/\.+$/, '');
  if (!domain || domain.length > 253 || /[/:?#@\[\]]/.test(domain)) return '';
  const labels = domain.split('.');
  if (labels.some((label) => !label
    || label.length > 63
    || !/^[a-z0-9-]+$/.test(label)
    || label.startsWith('-')
    || label.endsWith('-'))) return '';
  return domain;
}

export function normalizeTrustedDomains(values: readonly string[]) {
  return [...new Set(values.map(normalizeTrustedDomain).filter(Boolean))];
}

export function isTrustedExternalUrl(raw: string, trustedDomains: readonly string[]) {
  const value = normalizeHttpUrl(raw);
  if (!value) return false;
  const parsed = new URL(value);
  if (parsed.protocol !== 'https:') return false;
  const hostname = parsed.hostname.toLowerCase().replace(/\.+$/, '');
  return normalizeTrustedDomains(trustedDomains).some(
    (domain) => hostname === domain || hostname.endsWith(`.${domain}`),
  );
}

/**
 * Return a canonical cross-origin HTTP(S) URL, or an empty string when the
 * value is unsafe, relative, or still points at this site.
 */
export function normalizeExternalUrl(raw: string, currentOrigin = browserOrigin()) {
  const value = normalizeHttpUrl(raw);
  if (!value) return '';
  try {
    const url = new URL(value);
    if (currentOrigin && url.origin === new URL(currentOrigin).origin) return '';
    return url.href;
  } catch {
    return '';
  }
}

export function buildExternalWarningRoute(
  raw: string,
  currentOrigin = browserOrigin(),
  trustedDomains: readonly string[] = [],
): ExternalWarningRoute | null {
  const url = normalizeExternalUrl(raw, currentOrigin);
  return url && !isTrustedExternalUrl(url, trustedDomains)
    ? { name: EXTERNAL_LINK_ROUTE_NAME, query: { url } }
    : null;
}

export function resolveExternalWarningHref(
  router: ExternalLinkRouter,
  raw: string,
  currentOrigin = browserOrigin(),
  trustedDomains: readonly string[] = [],
) {
  const route = buildExternalWarningRoute(raw, currentOrigin, trustedDomains);
  return route ? router.resolve(route).href : '';
}

/** Route every programmatic external navigation through the local warning page. */
export function openExternalLink(
  router: ExternalLinkRouter,
  raw: string,
  options: OpenExternalLinkOptions = {},
) {
  const route = buildExternalWarningRoute(
    raw,
    options.currentOrigin,
    options.trustedDomains,
  );
  if (!route) return false;
  if (options.newTab) {
    const popup = window.open(
      router.resolve(route).href,
      '_blank',
      'noopener,noreferrer',
    );
    if (popup) popup.opener = null;
    return Boolean(popup);
  }
  void router.push(route);
  return true;
}
