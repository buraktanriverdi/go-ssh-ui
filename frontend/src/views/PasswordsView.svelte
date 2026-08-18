<script lang="ts">
  import { PasswordService } from "../../bindings/go-ssh-ui";
  import type { PasswordEntry } from "../../bindings/go-ssh/password/models";
  import { KeyRound, Eye, Pencil, Trash2, Copy, Plus } from "@lucide/svelte";
  import ConfirmDialog from "../components/ConfirmDialog.svelte";
  import { t } from "../lib/i18n/i18n.svelte";

  let confirmDialogRef: ReturnType<typeof ConfirmDialog> = $state()!;
  let entries: PasswordEntry[] = $state([]);
  let loading = $state(true);
  let loadError: string | null = $state(null);

  let editDialog: HTMLDialogElement;
  let revealDialog: HTMLDialogElement;

  let editingId: string | null = $state(null); // null => add mode
  let formId = $state("");
  let formDescription = $state("");
  let formPassword = $state("");
  let formError: string | null = $state(null);
  let busy = $state(false);

  let revealId = $state("");
  let revealValue = $state("");

  async function refresh() {
    try {
      entries = ((await PasswordService.List()) ?? []).filter((e): e is PasswordEntry => e !== null);
      loadError = null;
    } catch (err) {
      loadError = String(err);
    } finally {
      loading = false;
    }
  }
  refresh();

  function openAdd() {
    editingId = null;
    formId = "";
    formDescription = "";
    formPassword = "";
    formError = null;
    editDialog.showModal();
  }

  async function openEdit(entry: PasswordEntry) {
    editingId = entry.id;
    formId = entry.id;
    formDescription = entry.description;
    formError = null;
    try {
      formPassword = await PasswordService.Reveal(entry.id);
    } catch (err) {
      formPassword = "";
    }
    editDialog.showModal();
  }

  async function submitForm(e: Event) {
    e.preventDefault();
    formError = null;
    busy = true;
    try {
      if (editingId === null) {
        entries = ((await PasswordService.Add(formId, formDescription, formPassword)) ?? []).filter((e): e is PasswordEntry => e !== null);
      } else {
        entries = ((await PasswordService.Edit(editingId, formDescription, formPassword)) ?? []).filter((e): e is PasswordEntry => e !== null);
      }
      editDialog.close();
    } catch (err) {
      formError = String(err);
    } finally {
      busy = false;
    }
  }

  async function remove(entry: PasswordEntry) {
    const ok = await confirmDialogRef.open(t("passwordsView.confirmDelete", { id: entry.id }));
    if (!ok) return;
    entries = ((await PasswordService.Delete(entry.id)) ?? []).filter((e): e is PasswordEntry => e !== null);
  }

  async function reveal(entry: PasswordEntry) {
    revealId = entry.id;
    revealValue = await PasswordService.Reveal(entry.id);
    revealDialog.showModal();
  }

  function copyRevealed() {
    navigator.clipboard.writeText(revealValue);
  }
</script>

<div class="passwords-view">
  <div class="row end action-bar">
    <button type="button" class="primary" onclick={openAdd}>
      <Plus size={14} strokeWidth={2} />
      {t("passwordsView.buttons.add")}
    </button>
  </div>

  {#if loadError}
    <p class="error-text">{loadError}</p>
  {:else if loading}
    <p class="muted">{t("passwordsView.loading")}</p>
  {:else if entries.length === 0}
    <p class="muted">{t("passwordsView.emptyState")}</p>
  {:else}
    <div class="glass-panel entry-list">
      {#each entries as entry (entry.id)}
        <div class="list-row entry-row">
          <KeyRound size={14} strokeWidth={1.75} class="row-icon" />
          <div class="entry-info">
            <div class="mono entry-id">{entry.id}</div>
            <div class="muted">{entry.description}</div>
          </div>
          <span class="spacer"></span>
          <button type="button" class="icon-btn" onclick={() => reveal(entry)} title={t("passwordsView.actions.view")}>
            <Eye size={14} strokeWidth={2} />
          </button>
          <button type="button" class="icon-btn" onclick={() => openEdit(entry)} title={t("passwordsView.actions.edit")}>
            <Pencil size={13} strokeWidth={2} />
          </button>
          <button type="button" class="icon-btn danger" onclick={() => remove(entry)} title={t("passwordsView.actions.delete")}>
            <Trash2 size={13} strokeWidth={2} />
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>

<dialog bind:this={editDialog} class="glass-panel">
  <h3>{editingId === null ? t("passwordsView.dialog.addTitle") : t("passwordsView.dialog.editTitle")}</h3>
  <form onsubmit={submitForm}>
    <div class="field">
      <label for="pw-id">{t("passwordsView.dialog.idLabel")}</label>
      <input id="pw-id" type="text" bind:value={formId} disabled={editingId !== null} placeholder={t("passwordsView.dialog.idPlaceholder")} required />
    </div>
    <div class="field">
      <label for="pw-desc">{t("passwordsView.dialog.descriptionLabel")}</label>
      <input id="pw-desc" type="text" bind:value={formDescription} />
    </div>
    <div class="field">
      <label for="pw-value">{t("passwordsView.dialog.passwordLabel")}</label>
      <input id="pw-value" type="text" class="mono" bind:value={formPassword} required />
    </div>
    {#if formError}<p class="error-text">{formError}</p>{/if}
    <div class="row end">
      <button type="button" onclick={() => editDialog.close()}>{t("passwordsView.dialog.cancelButton")}</button>
      <button type="submit" class="primary" disabled={busy}>{t("passwordsView.dialog.saveButton")}</button>
    </div>
  </form>
</dialog>

<dialog bind:this={revealDialog} class="glass-panel">
  <h3>{t("passwordsView.revealDialog.title")}</h3>
  <p class="mono muted">{revealId}</p>
  <code class="reveal-box mono">{revealValue}</code>
  <div class="row end" style="margin-top: 16px">
    <button type="button" onclick={copyRevealed}>
      <Copy size={14} strokeWidth={2} />
      {t("passwordsView.revealDialog.copyButton")}
    </button>
    <button type="button" class="primary" onclick={() => revealDialog.close()}>{t("passwordsView.revealDialog.closeButton")}</button>
  </div>
</dialog>

<ConfirmDialog bind:this={confirmDialogRef} />

<style>
  .passwords-view {
    padding: 16px 20px 32px;
    height: 100%;
    overflow-y: auto;
  }
  .action-bar {
    margin-bottom: 12px;
  }
  .entry-list {
    display: flex;
    flex-direction: column;
    padding: 4px;
  }
  .entry-row {
    padding: 6px 8px;
  }
  :global(.row-icon) {
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .entry-info {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .entry-id {
    font-weight: 600;
  }
  .spacer {
    flex: 1;
  }
  .reveal-box {
    display: block;
    padding: 10px;
    background: var(--surface-bg-strong);
    border: 1px solid var(--surface-border);
    border-radius: var(--radius-sm);
    word-break: break-all;
  }
</style>
