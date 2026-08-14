<script lang="ts">
  import { onMount } from "svelte";
  import { Palette, RotateCcw, Check } from "@lucide/svelte";
  import { AppearanceService } from "../../bindings/go-ssh-ui";
  import { appearance, ACCENT_PRESETS, setOpacity, setBlur, setAccent, resetAppearance } from "../lib/appearance.svelte";

  function normalized(hex: string) {
    return hex.trim().toLowerCase();
  }

  // Unlike opacity/blur/accent above (plain CSS vars, applied live), this
  // one preference lives on the Go side and only takes effect on next
  // launch - see AppearanceService/windowprefs.go for why: Wails decides a
  // window's backdrop material at creation time, with no runtime setter.
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
