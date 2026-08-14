<script lang="ts">
  import { PasswordService } from "../../bindings/go-ssh-ui";
  import { Lock } from "@lucide/svelte";

  let { onUnlocked }: { onUnlocked: () => void } = $props();

  let storeExists: boolean | null = $state(null);
  let masterPassword = $state("");
  let confirmPassword = $state("");
  let error: string | null = $state(null);
  let busy = $state(false);

  async function refreshStatus() {
    const status = await PasswordService.Status();
    storeExists = status.storeExists;
  }
  refreshStatus();

  async function submitCreate(e: Event) {
    e.preventDefault();
    error = null;
    if (masterPassword.length < 8) {
      error = "Ana şifre en az 8 karakter olmalı.";
      return;
    }
    if (masterPassword !== confirmPassword) {
      error = "Şifreler eşleşmiyor.";
      return;
    }
    busy = true;
    try {
      await PasswordService.CreateStore(masterPassword);
      onUnlocked();
    } catch (err) {
      error = String(err);
    } finally {
      busy = false;
    }
  }

  async function submitUnlock(e: Event) {
    e.preventDefault();
    error = null;
    busy = true;
    try {
      await PasswordService.Unlock(masterPassword);
      onUnlocked();
    } catch (err) {
      error = "Ana şifre yanlış.";
    } finally {
      busy = false;
    }
  }
</script>

<div class="unlock-wrap">
  <div class="unlock-card glass-panel">
    <div class="unlock-badge"><Lock size={22} strokeWidth={1.75} /></div>
    <h1 class="unlock-title">go-ssh-ui</h1>
    {#if storeExists === null}
      <p class="muted">Yükleniyor…</p>
    {:else if storeExists === false}
      <p class="muted">Henüz bir şifre deposu yok. Bir ana şifre oluştur - bu şifre ~/.go-ssh/passwords.enc dosyasını korur ve go-ssh CLI ile de paylaşılır.</p>
      <form onsubmit={submitCreate}>
        <div class="field">
          <label for="create-pw">Ana şifre (en az 8 karakter)</label>
          <input id="create-pw" type="password" bind:value={masterPassword} autocomplete="new-password" />
        </div>
        <div class="field">
          <label for="confirm-pw">Ana şifre (tekrar)</label>
          <input id="confirm-pw" type="password" bind:value={confirmPassword} autocomplete="new-password" />
        </div>
        {#if error}<p class="error-text">{error}</p>{/if}
        <button type="submit" class="primary" disabled={busy}>Oluştur ve Aç</button>
      </form>
    {:else}
      <p class="muted">Devam etmek için ana şifreni gir.</p>
      <form onsubmit={submitUnlock}>
        <div class="field">
          <label for="unlock-pw">Ana şifre</label>
          <input id="unlock-pw" type="password" bind:value={masterPassword} autocomplete="current-password" autofocus />
        </div>
        {#if error}<p class="error-text">{error}</p>{/if}
        <button type="submit" class="primary" disabled={busy}>Kilidi Aç</button>
      </form>
    {/if}
  </div>
</div>

<style>
  .unlock-wrap {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    -webkit-app-region: drag;
  }
  .unlock-card {
    width: min(380px, 90vw);
    padding: 28px;
    text-align: center;
    -webkit-app-region: no-drag;
  }
  .unlock-badge {
    width: 48px;
    height: 48px;
    margin: 0 auto 12px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .unlock-title {
    margin: 0 0 4px;
    font-size: 17px;
  }
  form {
    text-align: left;
  }
  form {
    margin-top: 16px;
  }
  button {
    width: 100%;
  }
</style>
