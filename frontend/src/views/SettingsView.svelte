<script lang="ts">
  import { Palette, RotateCcw, Check } from "@lucide/svelte";
  import { appearance, ACCENT_PRESETS, setOpacity, setBlur, setAccent, resetAppearance } from "../lib/appearance.svelte";

  function normalized(hex: string) {
    return hex.trim().toLowerCase();
  }
</script>

<div class="settings-view">
  <h2>Ayarlar</h2>
  <p class="muted section-intro">
    Pencerenin saydamlığını, arka plan bulanıklığını ve renk tonunu buradan ayarlayabilirsin. Değişiklikler anında uygulanır ve
    bir sonraki açılışta hatırlanır.
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
        0, diyalog ve panellerdeki ek bulanıklığı tamamen kapatır (en keskin görünüm). Pencerenin arkasındaki hafif
        native macOS bulanıklığı (vibrancy) sabittir; bu sürgü onu değiştirmez, ama saydamlığı azaltıp opaklığı
        yükselttikçe daha az fark edilir.
      </p>
    </div>
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
