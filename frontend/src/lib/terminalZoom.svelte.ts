// Default terminal font size - an "Ayarlar" bulk setting that only seeds
// newly opened terminals. Each TerminalPane then owns its own font size
// locally from that point on: Cmd+=/Cmd+-/Cmd+0 and a pane's right-click
// menu resize only that one terminal, like Terminal.app/iTerm tabs, rather
// than every open terminal at once. Persisted to localStorage only
// (frontend-only preference, not written to ~/.go-ssh/ - see PLAN.md Faz 6).

export const DEFAULT_FONT_SIZE = 13;
export const FONT_SIZE_RANGE = [8, 32] as const;
const STORAGE_KEY = "go-ssh-ui.terminalFontSize";

export function clampFontSize(v: number) {
  return Math.min(FONT_SIZE_RANGE[1], Math.max(FONT_SIZE_RANGE[0], Math.round(v)));
}

export const terminalDefaults: { fontSize: number } = $state({ fontSize: DEFAULT_FONT_SIZE });

function persist() {
  localStorage.setItem(STORAGE_KEY, String(terminalDefaults.fontSize));
}

// Reads the persisted default (if any). Call once at boot, before mount, so
// the first terminal opened doesn't flash the hardcoded default then jump.
export function loadDefaultTerminalFontSize() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw !== null) {
      const parsed = Number(raw);
      if (Number.isFinite(parsed)) terminalDefaults.fontSize = clampFontSize(parsed);
    }
  } catch {
    // Malformed/foreign localStorage value - fall back to the default.
  }
}

export function setDefaultTerminalFontSize(v: number) {
  terminalDefaults.fontSize = clampFontSize(v);
  persist();
}
