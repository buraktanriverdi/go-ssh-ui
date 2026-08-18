<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Palette, RotateCcw, Check, KeyRound, PlayCircle } from "@lucide/svelte";
  import { AppearanceService, HotkeyWindowService } from "../../bindings/go-ssh-ui";
  import { WindowBackdrop, type HotkeyWindowSettings } from "../../bindings/go-ssh-ui";
  import { appearance, ACCENT_PRESETS, setOpacity, setBlur, setAccent, resetAppearance } from "../lib/appearance.svelte";
  import { terminalDefaults, FONT_SIZE_RANGE, setDefaultTerminalFontSize } from "../lib/terminalZoom.svelte";
  import { t, i18n, setLocale, type Locale } from "../lib/i18n/i18n.svelte";

  const LANGUAGE_OPTIONS: { value: Locale | "system"; labelKey: "settingsView.language.systemLabel" | "settingsView.language.turkishLabel" | "settingsView.language.englishLabel" }[] = [
    { value: "system", labelKey: "settingsView.language.systemLabel" },
    { value: "tr", labelKey: "settingsView.language.turkishLabel" },
    { value: "en", labelKey: "settingsView.language.englishLabel" },
  ];

  function normalized(hex: string) {
    return hex.trim().toLowerCase();
  }

  // Unlike opacity/blur/accent above (plain CSS vars, applied live), this
  // preference lives on the Go side and only takes effect on next launch -
  // see AppearanceService/windowprefs.go for why: Wails decides a window's
  // backdrop material at creation time, with no runtime setter. Shared by
  // both the main window's picker below and the hotkey window's (further
  // down) - same three choices, same restart caveat, different persisted
  // setting underneath (AppearanceService vs. HotkeyWindowService).
  const BACKDROP_OPTIONS: { value: string; labelKey: "settingsView.backdrop.options.translucent.label" | "settingsView.backdrop.options.liquidGlass.label" | "settingsView.backdrop.options.transparent.label"; hintKey: "settingsView.backdrop.options.translucent.hint" | "settingsView.backdrop.options.liquidGlass.hint" | "settingsView.backdrop.options.transparent.hint" }[] = [
    { value: "translucent", labelKey: "settingsView.backdrop.options.translucent.label", hintKey: "settingsView.backdrop.options.translucent.hint" },
    { value: "liquidGlass", labelKey: "settingsView.backdrop.options.liquidGlass.label", hintKey: "settingsView.backdrop.options.liquidGlass.hint" },
    { value: "transparent", labelKey: "settingsView.backdrop.options.transparent.label", hintKey: "settingsView.backdrop.options.transparent.hint" },
  ];

  let backdrop = $state("translucent");
  let backdropSaved = $state(false);
  let savedTimer: ReturnType<typeof setTimeout> | undefined;

  onMount(async () => {
    try {
      backdrop = await AppearanceService.GetBackdrop();
    } catch {
      // Leave the "translucent" default if the call fails for any reason.
    }
  });

  async function setBackdrop(value: string) {
    const previous = backdrop;
    backdrop = value;
    try {
      await AppearanceService.SetBackdrop(value);
      backdropSaved = true;
      clearTimeout(savedTimer);
      savedTimer = setTimeout(() => (backdropSaved = false), 3000);
    } catch {
      backdrop = previous;
    }
  }

  // --- Hotkey Penceresi (iTerm-style hotkey window) ---
  // Shortcut/color/opacity apply live with no restart - see
  // HotkeyWindowService.SetSettings. Backdrop is the one exception, same
  // "next launch" caveat as the main window's picker above (Wails only
  // picks a window's native material at creation time).
  let hotkey: HotkeyWindowSettings = $state({
    enabled: false,
    shortcut: "",
    bgColor: "#14141a",
    opacity: 90,
    backdrop: WindowBackdrop.BackdropTranslucent,
    height: 420,
  });
  let hotkeySaving = $state(false);
  let hotkeyError: string | null = $state(null);
  let hotkeySaved = $state(false);
  let hotkeySavedTimer: ReturnType<typeof setTimeout> | undefined;
  let hotkeyCapturing = $state(false);

  onMount(async () => {
    try {
      hotkey = await HotkeyWindowService.GetSettings();
    } catch {
      // Leave the defaults above if the call fails for any reason.
    }
  });

  async function saveHotkey(next: HotkeyWindowSettings) {
    const previous = hotkey;
    hotkey = next;
    hotkeySaving = true;
    hotkeyError = null;
    try {
      await HotkeyWindowService.SetSettings(next);
      hotkeySaved = true;
      clearTimeout(hotkeySavedTimer);
      hotkeySavedTimer = setTimeout(() => (hotkeySaved = false), 3000);
    } catch (e) {
      hotkey = previous;
      hotkeyError = e instanceof Error ? e.message : String(e);
    } finally {
      hotkeySaving = false;
    }
  }

  function toggleHotkeyEnabled() {
    saveHotkey({ ...hotkey, enabled: !hotkey.enabled });
  }

  function setHotkeyColor(hex: string) {
    saveHotkey({ ...hotkey, bgColor: hex });
  }

  function setHotkeyOpacity(v: number) {
    saveHotkey({ ...hotkey, opacity: v });
  }

  function setHotkeyBackdrop(value: string) {
    saveHotkey({ ...hotkey, backdrop: value as WindowBackdrop });
  }

  function testHotkeyWindow() {
    HotkeyWindowService.Toggle().catch(() => {});
  }

  // Maps browser KeyboardEvent.code (the physical key position, independent
  // of keyboard layout/Shift state) to the same key-name spelling Wails'
  // accelerator parser accepts (github.com/wailsapp/wails/v3 pkg/application
  // keys.go's `namedKeys`) - modifiers are handled separately below via the
  // event's own modifier flags.
  //
  // This must key off .code, not .key: Wails' macOS backend (Carbon's
  // RegisterEventHotKey, see global_shortcut_darwin.go's macKeyCodes) binds
  // *hardware* key codes and only recognizes plain, unshifted glyphs (e.g.
  // "'", never the shifted "\""). .key reflects the actual character the
  // active layout+Shift state produces - so on a Turkish layout, Shift+' can
  // come through as e.key === "\"" with no way to map it back to a
  // registerable key. .code reports the physical key itself (e.g. "Quote")
  // regardless of layout, which is exactly what the backend expects; a
  // Shift-combo then comes through correctly as a Shift *modifier* plus the
  // base key (e.g. Cmd+Shift+2) instead of an unsupported literal symbol.
  const CODE_KEY_MAP: Record<string, string> = {
    Backquote: "`",
    Minus: "-",
    Equal: "=",
    BracketLeft: "[",
    BracketRight: "]",
    Backslash: "\\",
    Semicolon: ";",
    Quote: "'",
    Comma: ",",
    Period: ".",
    Slash: "/",
    Backspace: "backspace",
    Tab: "tab",
    Enter: "enter",
    Escape: "escape",
    ArrowLeft: "left",
    ArrowRight: "right",
    ArrowUp: "up",
    ArrowDown: "down",
    Space: "space",
    Delete: "delete",
    Home: "home",
    End: "end",
    PageUp: "page up",
    PageDown: "page down",
    NumLock: "numlock",
  };
  const IGNORED_CAPTURE_CODES = new Set([
    "MetaLeft",
    "MetaRight",
    "ControlLeft",
    "ControlRight",
    "AltLeft",
    "AltRight",
    "ShiftLeft",
    "ShiftRight",
    "CapsLock",
  ]);
  // macKeyCodes on the Go side only goes up to f20 - matching that here
  // means an unsupported F21+ key hits the same friendly error below
  // instead of a confusing round-trip failure from the backend.
  const CODE_FUNCTION_KEY = /^F([1-9]|1\d|20)$/;
  const FUNCTION_KEY = /^f([1-9]|1\d|20)$/;

  function prettyKey(key: string): string {
    if (key.length === 1) return key.toUpperCase();
    return key.replace(/\b\w/g, (c) => c.toUpperCase());
  }

  function onCaptureKeydown(e: KeyboardEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (e.key === "Escape") {
      stopCapture();
      return;
    }
    if (IGNORED_CAPTURE_CODES.has(e.code)) return;

    let key: string | null = null;
    if (CODE_KEY_MAP[e.code]) key = CODE_KEY_MAP[e.code];
    else if (CODE_FUNCTION_KEY.test(e.code)) key = e.code.toLowerCase();
    else if (/^Key[A-Z]$/.test(e.code)) key = e.code.slice(3).toLowerCase();
    else if (/^Digit[0-9]$/.test(e.code)) key = e.code.slice(5);
    if (!key) {
      // Genuinely unsupported physical key - e.g. the ISO-only extra key
      // next to Enter/Quote some non-US keyboards have (code
      // "IntlBackslash", producing '"' on a Turkish layout): Wails' macOS
      // backend (Carbon's RegisterEventHotKey) only has virtual-keycode
      // mappings for standard ANSI-position keys, so this can't be
      // registered as a global shortcut no matter what modifiers are held.
      hotkeyError = t("settingsView.hotkey.errors.unsupportedKey");
      return;
    }

    const modifiers: string[] = [];
    if (e.metaKey) modifiers.push("Cmd");
    if (e.ctrlKey) modifiers.push("Ctrl");
    if (e.altKey) modifiers.push("Option");
    if (e.shiftKey) modifiers.push("Shift");

    // A global shortcut with no modifier at all (bare "A", bare Space...)
    // would swallow that key everywhere, all the time - only function keys
    // are common/safe enough to allow unmodified.
    if (modifiers.length === 0 && !FUNCTION_KEY.test(key)) {
      hotkeyError = t("settingsView.hotkey.errors.needsModifier");
      return;
    }

    stopCapture();
    saveHotkey({ ...hotkey, shortcut: [...modifiers, prettyKey(key)].join("+"), enabled: true });
  }

  function startCapture() {
    hotkeyCapturing = true;
    hotkeyError = null;
    window.addEventListener("keydown", onCaptureKeydown, { capture: true });
  }

  function stopCapture() {
    hotkeyCapturing = false;
    window.removeEventListener("keydown", onCaptureKeydown, { capture: true });
  }

  onDestroy(() => window.removeEventListener("keydown", onCaptureKeydown, { capture: true }));
</script>

<div class="settings-view">
  <h2>{t("settingsView.title")}</h2>
  <p class="muted section-intro">{t("settingsView.intro")}</p>

  <div class="glass-panel settings-card">
    <h3>{t("settingsView.appearance.heading")}</h3>

    <div class="field">
      <div class="row between">
        <label for="opacity-range">{t("settingsView.appearance.opacityLabel")}</label>
        <span class="muted mono value-badge">%{appearance.opacity}</span>
      </div>
      <input
        id="opacity-range"
        type="range"
        min="5"
        max="100"
        step="1"
        value={appearance.opacity}
        oninput={(e) => setOpacity(Number(e.currentTarget.value))}
      />
      <p class="muted hint">{t("settingsView.appearance.opacityHint")}</p>
    </div>

    <div class="field">
      <div class="row between">
        <label for="blur-range">{t("settingsView.appearance.blurLabel")}</label>
        <span class="muted mono value-badge">{appearance.blur}px</span>
      </div>
      <input
        id="blur-range"
        type="range"
        min="0"
        max="40"
        step="1"
        value={appearance.blur}
        oninput={(e) => setBlur(Number(e.currentTarget.value))}
      />
      <p class="muted hint">{t("settingsView.appearance.blurHint")}</p>
    </div>
  </div>

  <div class="glass-panel settings-card">
    <h3>{t("settingsView.terminal.heading")}</h3>

    <div class="field">
      <div class="row between">
        <label for="terminal-font-size-range">{t("settingsView.terminal.fontSizeLabel")}</label>
        <span class="muted mono value-badge">{terminalDefaults.fontSize}px</span>
      </div>
      <input
        id="terminal-font-size-range"
        type="range"
        min={FONT_SIZE_RANGE[0]}
        max={FONT_SIZE_RANGE[1]}
        step="1"
        value={terminalDefaults.fontSize}
        oninput={(e) => setDefaultTerminalFontSize(Number(e.currentTarget.value))}
      />
      <p class="muted hint">{t("settingsView.terminal.fontSizeHint")}</p>
    </div>
  </div>

  <div class="glass-panel settings-card">
    <h3>{t("settingsView.language.heading")}</h3>
    <p class="muted section-intro">{t("settingsView.language.intro")}</p>
    <div class="backdrop-options">
      {#each LANGUAGE_OPTIONS as opt (opt.value)}
        <label class="backdrop-option {(i18n.isSystem && opt.value === 'system') || (!i18n.isSystem && opt.value === i18n.locale) ? 'selected' : ''}">
          <input
            type="radio"
            name="language"
            value={opt.value}
            checked={(i18n.isSystem && opt.value === "system") || (!i18n.isSystem && opt.value === i18n.locale)}
            onchange={() => setLocale(opt.value)}
          />
          <span class="backdrop-option-text">
            <span class="backdrop-option-label">{t(opt.labelKey)}</span>
            {#if opt.value === "system"}<span class="muted backdrop-option-hint">{t("settingsView.language.systemHint")}</span>{/if}
          </span>
        </label>
      {/each}
    </div>
  </div>

  <div class="glass-panel settings-card">
    <h3>{t("settingsView.backdrop.heading")}</h3>
    <div class="backdrop-options">
      {#each BACKDROP_OPTIONS as opt (opt.value)}
        <label class="backdrop-option {backdrop === opt.value ? 'selected' : ''}">
          <input type="radio" name="backdrop" value={opt.value} checked={backdrop === opt.value} onchange={() => setBackdrop(opt.value)} />
          <span class="backdrop-option-text">
            <span class="backdrop-option-label">{t(opt.labelKey)}</span>
            <span class="muted backdrop-option-hint">{t(opt.hintKey)}</span>
          </span>
        </label>
      {/each}
    </div>
    <p class="muted hint">{t("settingsView.backdrop.hintDiff")}</p>
    <p class="muted hint">{t("settingsView.backdrop.hintRestart")}</p>
    {#if backdropSaved}<p class="saved-hint">{t("settingsView.backdrop.saved")}</p>{/if}
  </div>

  <div class="glass-panel settings-card">
    <h3>{t("settingsView.hotkey.heading")}</h3>
    <p class="muted section-intro hotkey-intro">{t("settingsView.hotkey.intro")}</p>

    <label class="hotkey-enable-row">
      <input type="checkbox" checked={hotkey.enabled} onchange={toggleHotkeyEnabled} disabled={hotkeySaving} />
      <span>{t("settingsView.hotkey.enabled")}</span>
    </label>

    <div class="field">
      <label for="hotkey-shortcut-btn">{t("settingsView.hotkey.shortcutLabel")}</label>
      <div class="row hotkey-shortcut-row">
        <span class="mono hotkey-shortcut-value">{hotkey.shortcut || t("settingsView.hotkey.shortcutUnset")}</span>
        <button id="hotkey-shortcut-btn" type="button" class="ghost" onclick={hotkeyCapturing ? stopCapture : startCapture}>
          <KeyRound size={13} strokeWidth={2} />
          {hotkeyCapturing ? t("settingsView.hotkey.capturing") : t("settingsView.hotkey.changeShortcut")}
        </button>
      </div>
      {#if hotkeyError}<p class="error-text hint">{hotkeyError}</p>{/if}
      {#if hotkey.enabled && !hotkey.shortcut}
        <p class="muted hint">{t("settingsView.hotkey.unsetHint")}</p>
      {/if}
    </div>

    <div class="field">
      <div class="row between">
        <label for="hotkey-opacity">{t("settingsView.hotkey.opacityLabel")}</label>
        <span class="muted mono value-badge">%{hotkey.opacity}</span>
      </div>
      <input
        id="hotkey-opacity"
        type="range"
        min="10"
        max="100"
        step="1"
        value={hotkey.opacity}
        oninput={(e) => setHotkeyOpacity(Number(e.currentTarget.value))}
      />
    </div>

    <div class="field">
      <label for="hotkey-color">{t("settingsView.hotkey.colorLabel")}</label>
      <input id="hotkey-color" type="color" value={hotkey.bgColor} oninput={(e) => setHotkeyColor(e.currentTarget.value)} />
    </div>

    <div class="field">
      <label for="hotkey-backdrop-options">{t("settingsView.hotkey.backdropLabel")}</label>
      <div id="hotkey-backdrop-options" class="backdrop-options">
        {#each BACKDROP_OPTIONS as opt (opt.value)}
          <label class="backdrop-option {hotkey.backdrop === opt.value ? 'selected' : ''}">
            <input
              type="radio"
              name="hotkey-backdrop"
              value={opt.value}
              checked={hotkey.backdrop === opt.value}
              onchange={() => setHotkeyBackdrop(opt.value)}
            />
            <span class="backdrop-option-text">
              <span class="backdrop-option-label">{t(opt.labelKey)}</span>
              <span class="muted backdrop-option-hint">{t(opt.hintKey)}</span>
            </span>
          </label>
        {/each}
      </div>
      <p class="muted hint">{t("settingsView.hotkey.backdropHint")}</p>
    </div>

    <div class="row between">
      <button type="button" class="ghost" onclick={testHotkeyWindow}>
        <PlayCircle size={13} strokeWidth={2} />
        {t("settingsView.hotkey.testButton")}
      </button>
      {#if hotkeySaved}<span class="saved-hint">{t("settingsView.hotkey.saved")}</span>{/if}
    </div>
    <p class="muted hint">{t("settingsView.hotkey.liveHint")}</p>
  </div>

  <div class="glass-panel settings-card">
    <h3>{t("settingsView.accent.heading")}</h3>
    <div class="accent-grid">
      {#each ACCENT_PRESETS as preset (preset.color)}
        <button
          type="button"
          class="accent-swatch"
          style:background={preset.color}
          title={t(preset.nameKey)}
          aria-label={t(preset.nameKey)}
          onclick={() => setAccent(preset.color)}
        >
          {#if normalized(appearance.accent) === preset.color}
            <Check size={14} strokeWidth={3} color="#ffffff" />
          {/if}
        </button>
      {/each}
      <label class="accent-swatch accent-custom" style:background={appearance.accent} title={t("settingsView.accent.customTitle")}>
        <input type="color" value={appearance.accent} oninput={(e) => setAccent(e.currentTarget.value)} />
        <Palette size={13} strokeWidth={2} color="#ffffff" />
      </label>
    </div>
  </div>

  <button type="button" class="ghost reset-btn" onclick={resetAppearance}>
    <RotateCcw size={13} strokeWidth={2} />
    {t("settingsView.resetButton")}
  </button>
</div>

<style>
  .settings-view {
    padding: 16px 20px 32px;
    height: 100%;
    overflow-y: auto;
    max-width: 480px;
  }
  .section-intro {
    margin: 0 0 16px;
    font-size: 12.5px;
  }
  .settings-card {
    padding: 16px;
    margin-bottom: 14px;
  }
  .settings-card h3 {
    margin-bottom: 12px;
  }
  .value-badge {
    font-size: 11.5px;
  }
  input[type="range"] {
    width: 100%;
  }
  .hint {
    font-size: 11px;
    margin: 6px 0 0;
  }
  .backdrop-options {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .backdrop-option {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 6px 8px;
    margin: 0 -8px;
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .backdrop-option:hover {
    background: var(--row-hover);
  }
  .backdrop-option input {
    width: auto;
    margin-top: 2px;
  }
  .backdrop-option-text {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .backdrop-option-label {
    /* Global `label { color: var(--text-muted) }` would otherwise leave
       this looking as de-emphasized as the hint line below it. */
    color: var(--text);
    font-size: 12.5px;
    font-weight: 500;
  }
  .backdrop-option-hint {
    font-size: 11px;
  }
  .saved-hint {
    font-size: 11px;
    margin: 6px 0 0;
    color: var(--accent);
  }
  .hotkey-intro {
    margin: 0 0 12px;
  }
  .hotkey-enable-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
  }
  .hotkey-enable-row span {
    color: var(--text);
    font-size: 12.5px;
    font-weight: 500;
  }
  .hotkey-shortcut-row {
    justify-content: space-between;
  }
  .hotkey-shortcut-value {
    font-size: 12.5px;
    background: var(--surface-bg-strong);
    border: 1px solid var(--surface-border);
    border-radius: var(--radius-sm);
    padding: 5px 10px;
    flex: 1;
  }
  #hotkey-color {
    width: 48px;
    height: 28px;
    padding: 2px;
  }
  .accent-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }
  .accent-swatch {
    width: 30px;
    height: 30px;
    padding: 0;
    border-radius: 50%;
    border: 1px solid rgba(0, 0, 0, 0.15);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.2);
    cursor: pointer;
  }
  .accent-custom {
    /* Its `style:background` (bound to the current accent) always wins over
       any background-image/-size declared here, since an inline `background`
       shorthand implicitly resets those sub-properties too - so this rule
       only needs to handle the color-input overlay's positioning. */
    position: relative;
    overflow: hidden;
  }
  .accent-custom input[type="color"] {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
    padding: 0;
    border: none;
  }
  .reset-btn {
    font-size: 12px;
  }
</style>
