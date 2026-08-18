<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Palette, RotateCcw, Check, KeyRound, PlayCircle } from "@lucide/svelte";
  import { AppearanceService, HotkeyWindowService } from "../../bindings/go-ssh-ui";
  import { WindowBackdrop, type HotkeyWindowSettings } from "../../bindings/go-ssh-ui";
  import { appearance, ACCENT_PRESETS, setOpacity, setBlur, setAccent, resetAppearance } from "../lib/appearance.svelte";

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
  const BACKDROP_OPTIONS: { value: string; label: string; hint: string }[] = [
    { value: "translucent", label: "Bulanık (vibrancy)", hint: "Klasik frosted-glass macOS görünümü." },
    {
      value: "liquidGlass",
      label: "Liquid Glass",
      hint: "Apple'ın yeni camsı efekti (macOS 15+; eski sürümlerde otomatik Bulanık'a döner).",
    },
    { value: "transparent", label: "Şeffaf (bulanıksız)", hint: "Aynı derecede saydam, ama keskin - hiç bulanıklık yok." },
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
      hotkeyError = "Bu tuş sistem geneli kısayol olarak desteklenmiyor. Lütfen farklı bir tuş deneyin (harf, rakam veya fonksiyon tuşu önerilir).";
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
      hotkeyError = "Bu kısayol için en az bir değiştirici tuş (Cmd/Ctrl/Option/Shift) gerekli.";
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
  <h2>Ayarlar</h2>
  <p class="muted section-intro">
    Pencerenin saydamlığını, bulanıklığını ve renk tonunu buradan ayarlayabilirsin. Saydamlık/bulanıklık/renk anında
    uygulanır; native pencere bulanıklığı bir sonraki açılışta devreye girer.
  </p>

  <div class="glass-panel settings-card">
    <h3>Görünüm</h3>

    <div class="field">
      <div class="row between">
        <label for="opacity-range">Arka plan saydamlığı</label>
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
      <p class="muted hint">Düşük değer pencere arkasının (ve terminal arkaplanının) daha fazla görünmesini sağlar.</p>
    </div>

    <div class="field">
      <div class="row between">
        <label for="blur-range">Panel bulanıklığı</label>
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
      <p class="muted hint">
        0, diyalog ve panellerdeki ek bulanıklığı tamamen kapatır (en keskin görünüm). Pencerenin tüm arkaplanını
        etkileyen native bulanıklık ayrı bir ayar - aşağıda.
      </p>
    </div>
  </div>

  <div class="glass-panel settings-card">
    <h3>Pencere arkaplanı (native)</h3>
    <div class="backdrop-options">
      {#each BACKDROP_OPTIONS as opt (opt.value)}
        <label class="backdrop-option {backdrop === opt.value ? 'selected' : ''}">
          <input type="radio" name="backdrop" value={opt.value} checked={backdrop === opt.value} onchange={() => setBackdrop(opt.value)} />
          <span class="backdrop-option-text">
            <span class="backdrop-option-label">{opt.label}</span>
            <span class="muted backdrop-option-hint">{opt.hint}</span>
          </span>
        </label>
      {/each}
    </div>
    <p class="muted hint">
      Yukarıdaki "Panel bulanıklığı"ndan farklı: diyalog kartları değil, pencerenin arkasında görünen
      masaüstünü/diğer pencereleri etkileyen katman bu - hepsi üstteki saydamlık ayarına uyar.
    </p>
    <p class="muted hint">Değişikliğin görünmesi için uygulamayı kapatıp yeniden açman gerekiyor.</p>
    {#if backdropSaved}<p class="saved-hint">Kaydedildi - bir sonraki açılışta uygulanacak.</p>{/if}
  </div>

  <div class="glass-panel settings-card">
    <h3>Hotkey Penceresi</h3>
    <p class="muted section-intro hotkey-intro">
      iTerm'deki gibi: bir global kısayolla, uygulama arka plandayken bile açılıp kapanan, tek bir yerel terminal
      içeren yüzen bir pencere.
    </p>

    <label class="hotkey-enable-row">
      <input type="checkbox" checked={hotkey.enabled} onchange={toggleHotkeyEnabled} disabled={hotkeySaving} />
      <span>Etkin</span>
    </label>

    <div class="field">
      <label for="hotkey-shortcut-btn">Kısayol</label>
      <div class="row hotkey-shortcut-row">
        <span class="mono hotkey-shortcut-value">{hotkey.shortcut || "Ayarlanmadı"}</span>
        <button id="hotkey-shortcut-btn" type="button" class="ghost" onclick={hotkeyCapturing ? stopCapture : startCapture}>
          <KeyRound size={13} strokeWidth={2} />
          {hotkeyCapturing ? "Tuşlara basın… (iptal: Esc)" : "Kısayolu değiştir"}
        </button>
      </div>
      {#if hotkeyError}<p class="error-text hint">{hotkeyError}</p>{/if}
      {#if hotkey.enabled && !hotkey.shortcut}
        <p class="muted hint">Etkin ama henüz bir kısayol seçilmedi - "Kısayolu değiştir"e tıklayıp bir tuş kombinasyonuna bas.</p>
      {/if}
    </div>

    <div class="field">
      <div class="row between">
        <label for="hotkey-opacity">Pencere saydamlığı</label>
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
      <label for="hotkey-color">Pencere arka plan rengi</label>
      <input id="hotkey-color" type="color" value={hotkey.bgColor} oninput={(e) => setHotkeyColor(e.currentTarget.value)} />
    </div>

    <div class="field">
      <label for="hotkey-backdrop-options">Pencere arkaplanı (native)</label>
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
              <span class="backdrop-option-label">{opt.label}</span>
              <span class="muted backdrop-option-hint">{opt.hint}</span>
            </span>
          </label>
        {/each}
      </div>
      <p class="muted hint">Yukarıdaki renk/saydamlık bunun üzerine biner. Değişikliğin görünmesi için uygulamayı kapatıp yeniden açman gerekiyor.</p>
    </div>

    <div class="row between">
      <button type="button" class="ghost" onclick={testHotkeyWindow}>
        <PlayCircle size={13} strokeWidth={2} />
        Pencereyi test et
      </button>
      {#if hotkeySaved}<span class="saved-hint">Kaydedildi</span>{/if}
    </div>
    <p class="muted hint">Kısayol/renk/saydamlık anında uygulanır - pencere arkaplanı (native) hariç, yeniden başlatma gerekmez.</p>
  </div>

  <div class="glass-panel settings-card">
    <h3>Renk tonu</h3>
    <div class="accent-grid">
      {#each ACCENT_PRESETS as preset (preset.color)}
        <button
          type="button"
          class="accent-swatch"
          style:background={preset.color}
          title={preset.label}
          aria-label={preset.label}
          onclick={() => setAccent(preset.color)}
        >
          {#if normalized(appearance.accent) === preset.color}
            <Check size={14} strokeWidth={3} color="#ffffff" />
          {/if}
        </button>
      {/each}
      <label class="accent-swatch accent-custom" style:background={appearance.accent} title="Özel renk seç">
        <input type="color" value={appearance.accent} oninput={(e) => setAccent(e.currentTarget.value)} />
        <Palette size={13} strokeWidth={2} color="#ffffff" />
      </label>
    </div>
  </div>

  <button type="button" class="ghost reset-btn" onclick={resetAppearance}>
    <RotateCcw size={13} strokeWidth={2} />
    Varsayılanlara dön
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
