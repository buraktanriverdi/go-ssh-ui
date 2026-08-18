// Multi-language runtime: current locale as reactive $state (same pattern as
// appearance.svelte.ts - a plain state object + apply/persist functions,
// loaded once at boot). Default locale is read from the OS via
// navigator.language, which WKWebView/WebView2/webkitgtk all report as the
// system UI language - Wails' own runtime exposes no locale API (see
// @wailsio/runtime's system.d.ts). A user override (set via SettingsView)
// wins over the system default and is persisted to localStorage, mirroring
// appearance's "frontend-only preference, not written to ~/.go-ssh/" choice.
import { tr, type Dict } from "./locales/tr";
import { en } from "./locales/en";

export type Locale = "tr" | "en";

// Recursively turns the nested Dict shape into a union of dot-path string
// keys (e.g. "app.home" | "unlockScreen.errors.tooShort" | ...) so every
// t(...) call site is checked against the actual translation tree at compile
// time - a typo'd or removed key fails `npm run check`, not just at runtime.
type DotPaths<T, Prefix extends string = ""> = {
  [K in keyof T & string]: T[K] extends string
    ? `${Prefix}${K}`
    : T[K] extends Record<string, unknown>
      ? DotPaths<T[K], `${Prefix}${K}.`>
      : never;
}[keyof T & string];

export type TranslationKey = DotPaths<Dict>;

const dictionaries: Record<Locale, Dict> = { tr, en };
const STORAGE_KEY = "go-ssh-ui.locale";

function detectSystemLocale(): Locale {
  const langs = navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language];
  for (const lang of langs) {
    if (lang?.toLowerCase().startsWith("tr")) return "tr";
  }
  return "en";
}

export const i18n: { locale: Locale; isSystem: boolean } = $state({
  locale: detectSystemLocale(),
  isSystem: true,
});

function applyDocumentLang() {
  document.documentElement.lang = i18n.locale;
}

// Reads a persisted override (if any) and applies it; call once at boot,
// before mount, so there's no flash of the wrong language.
export function loadLocale() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === "tr" || saved === "en") {
      i18n.locale = saved;
      i18n.isSystem = false;
      applyDocumentLang();
      return;
    }
  } catch {
    // Malformed/inaccessible localStorage - fall through to system default.
  }
  i18n.locale = detectSystemLocale();
  i18n.isSystem = true;
  applyDocumentLang();
}

// Pass "system" to clear the override and follow the OS language again.
export function setLocale(next: Locale | "system") {
  if (next === "system") {
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch {
      // Ignore - worst case the previous override just doesn't clear.
    }
    i18n.locale = detectSystemLocale();
    i18n.isSystem = true;
    applyDocumentLang();
    return;
  }
  i18n.locale = next;
  i18n.isSystem = false;
  applyDocumentLang();
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // Ignore - the in-memory switch still applies for this session.
  }
}

function resolve(dict: Dict, key: string): string | undefined {
  let node: unknown = dict;
  for (const part of key.split(".")) {
    if (typeof node !== "object" || node === null) return undefined;
    node = (node as Record<string, unknown>)[part];
  }
  return typeof node === "string" ? node : undefined;
}

// Falls back to the Turkish source string if a key is somehow missing from
// the active dictionary (should only happen mid-refactor, never at runtime
// once `npm run check` is clean, since Dict typing requires every namespace
// to exist in both locales).
export function t(key: TranslationKey, vars?: Record<string, string | number>): string {
  let value = resolve(dictionaries[i18n.locale], key) ?? resolve(tr, key) ?? key;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) value = value.split(`{${k}}`).join(String(v));
  }
  return value;
}
