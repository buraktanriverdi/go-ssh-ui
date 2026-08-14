// Shared "Görünüm" (appearance) state: background transparency, glass blur
// radius, and accent color. All three are plain CSS custom properties on
// <html> so every existing --app-bg/--glass-blur/--accent consumer in
// glass.css picks up changes live with no per-component wiring. Persisted to
// localStorage only (frontend-only preference, not written to ~/.go-ssh/ -
// see PLAN.md Faz 6).

export type Appearance = {
  opacity: number; // 0-100, drives --app-bg-alpha
  blur: number; // px, drives --glass-blur
  accent: string; // hex, drives --accent (+ derived --accent-contrast)
};

const DEFAULTS: Appearance = { opacity: 96, blur: 24, accent: "#007aff" };
const STORAGE_KEY = "go-ssh-ui.appearance";
// Opacity floor is intentionally low (near fully see-through) so the "more
// transparent" end of the slider actually gets there; blur floor is already
// 0 (= no .glass-panel blur at all, its true minimum - see SettingsView's
// hint about the native macOS vibrancy layer that lives outside this range).
const OPACITY_RANGE = [5, 100] as const;
const BLUR_RANGE = [0, 40] as const;

export const ACCENT_PRESETS: { label: string; color: string }[] = [
  { label: "Mavi", color: "#007aff" },
  { label: "Mor", color: "#af52de" },
  { label: "Pembe", color: "#ff2d55" },
  { label: "Kırmızı", color: "#ff3b30" },
  { label: "Turuncu", color: "#ff9500" },
  { label: "Sarı", color: "#ffcc00" },
  { label: "Yeşil", color: "#34c759" },
  { label: "Grafit", color: "#8e8e93" },
];

export const appearance: Appearance = $state({ ...DEFAULTS });

function clamp(v: number, [min, max]: readonly [number, number]) {
  return Math.min(max, Math.max(min, Math.round(v)));
}

// WCAG relative luminance -> pick black or white label text so a light
// accent pick (e.g. yellow) never ends up with unreadable white-on-white,
// the same class of bug this whole change is fixing for form controls.
function contrastText(hex: string): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return "#ffffff";
  const [r, g, b] = [m[1].slice(0, 2), m[1].slice(2, 4), m[1].slice(4, 6)].map((h) => parseInt(h, 16) / 255);
  const lin = (v: number) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
  const luminance = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
  return luminance > 0.45 ? "#1a1a1a" : "#ffffff";
}

function apply() {
  const root = document.documentElement.style;
  root.setProperty("--app-bg-alpha", String(appearance.opacity / 100));
  root.setProperty("--glass-blur", `${appearance.blur}px`);
  root.setProperty("--accent", appearance.accent);
  root.setProperty("--accent-contrast", contrastText(appearance.accent));
}

function persist() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(appearance));
}

// Reads persisted prefs (if any) and applies CSS vars. Call once at boot,
// before mount, so there's no flash of default appearance.
export function loadAppearance() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (typeof parsed.opacity === "number") appearance.opacity = clamp(parsed.opacity, OPACITY_RANGE);
      if (typeof parsed.blur === "number") appearance.blur = clamp(parsed.blur, BLUR_RANGE);
      if (typeof parsed.accent === "string" && /^#[0-9a-f]{6}$/i.test(parsed.accent)) appearance.accent = parsed.accent;
    }
  } catch {
    // Malformed/foreign localStorage value - fall back to defaults.
  }
  apply();
}

export function setOpacity(v: number) {
  appearance.opacity = clamp(v, OPACITY_RANGE);
  apply();
  persist();
}

export function setBlur(v: number) {
  appearance.blur = clamp(v, BLUR_RANGE);
  apply();
  persist();
}

export function setAccent(hex: string) {
  appearance.accent = hex;
  apply();
  persist();
}

export function resetAppearance() {
  Object.assign(appearance, DEFAULTS);
  apply();
  persist();
}
