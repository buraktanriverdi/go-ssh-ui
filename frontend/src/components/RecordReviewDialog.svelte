<script lang="ts">
  import { onMount } from "svelte";
  import { Eye, EyeOff, Trash2 } from "@lucide/svelte";
  import { HostService, PasswordService } from "../../bindings/go-ssh-ui";
  import type { Category, Host } from "../../bindings/go-ssh/config/models";
  import type { PasswordEntry } from "../../bindings/go-ssh/password/models";
  import type { Files } from "../../bindings/go-ssh-ui/internal/configx/models";
  import type { Snapshot, Step } from "../../bindings/go-ssh-ui/internal/record/models";
  import TargetFilePicker from "./TargetFilePicker.svelte";
  import { t } from "../lib/i18n/i18n.svelte";

  // Faz 5 "Kayıt": TerminalPane opens this once recording stops and at
  // least one step was captured. Saves the host directly (category + target
  // file + any new password entries) rather than handing prefilled data off
  // to the existing "Host Ekle" form under a category - the first version
  // did that, and it turned out to be a dead end in practice: users expect
  // the "Host Oluştur" button to actually create the host, not stage it for
  // a second manual step they have no reason to know about. Never saves
  // anything silently, though - everything below is editable, and nothing
  // is written until the user submits. Closing this any way (submit,
  // Vazgeç, Escape) always calls onDone exactly once, via the dialog's own
  // `close` event rather than duplicating the call at each call site.
  let { snapshot, onDone, onSaved }: { snapshot: Snapshot; onDone: () => void; onSaved?: () => void } = $props();

  let dialog: HTMLDialogElement;

  function lastToken(cmd: string): string {
    const tokens = cmd.trim().split(/\s+/);
    const last = tokens[tokens.length - 1] ?? "";
    const at = last.lastIndexOf("@");
    return at >= 0 ? last.slice(at + 1) : last;
  }

  function slug(s: string): string {
    return (
      s
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "") || "kayit"
    );
  }

  const initialSteps: Step[] = (snapshot.steps ?? []).filter((s): s is Step => s !== null);
  let name = $state(lastToken(initialSteps.find((s) => s.kind === "exec")?.value ?? ""));

  const NEW = "__new__";
  type Row =
    | { kind: "exec"; value: string }
    | { kind: "sendpass"; label: string; secret: string; choice: string; newId: string; newDesc: string; reveal: boolean };

  let passSeq = 0;
  let rows: Row[] = $state(
    initialSteps.map((s) =>
      s.kind === "exec"
        ? { kind: "exec", value: s.value }
        : {
            kind: "sendpass",
            label: s.label,
            secret: s.secret,
            choice: NEW,
            newId: `${slug(name)}-${++passSeq}`,
            newDesc: s.label,
            reveal: false,
          },
    ),
  );

  function removeRow(i: number) {
    rows = rows.filter((_, idx) => idx !== i);
  }

  let passwords: PasswordEntry[] = $state([]);
  let files: Files | null = $state(null);
  let categoryOptions: { path: string[]; label: string }[] = $state([]);
  let categoryChoice = $state("");
  let targetFile = $state("");

  let loading = $state(true);
  let busy = $state(false);
  let error: string | null = $state(null);

  function flattenCategories(categories: Category[] | null | undefined, prefix: string[] = []): { path: string[]; label: string }[] {
    let out: { path: string[]; label: string }[] = [];
    for (const cat of categories ?? []) {
      const path = [...prefix, cat.name];
      out.push({ path, label: path.join(" / ") });
      out = out.concat(flattenCategories(cat.categories, path));
    }
    return out;
  }

  onMount(async () => {
    dialog.showModal();
    try {
      const [entriesResult, filesResult, treeResult] = await Promise.all([
        PasswordService.List(),
        HostService.GetFiles(),
        HostService.GetTree(),
      ]);
      const entries = (entriesResult ?? []).filter((e): e is PasswordEntry => e !== null);
      passwords = entries;
      files = filesResult;
      categoryOptions = flattenCategories(treeResult?.categories);
      if (categoryOptions.length > 0) categoryChoice = "0";

      // Dedup: pre-select an existing entry for any step whose captured
      // value exactly matches an already-stored password.
      for (const row of rows) {
        if (row.kind !== "sendpass") continue;
        for (const entry of entries) {
          try {
            const value = await PasswordService.Reveal(entry.id);
            if (value === row.secret) {
              row.choice = entry.id;
              break;
            }
          } catch {
            // Entry vanished or reveal failed - leave this row on "yeni".
          }
        }
      }
    } finally {
      loading = false;
    }
  });

  async function submit(e: Event) {
    e.preventDefault();
    error = null;
    busy = true;
    try {
      if (rows.length === 0) throw new Error(t("recordReviewDialog.errors.noSteps"));
      const category = categoryOptions[Number(categoryChoice)];
      if (!category) throw new Error(t("recordReviewDialog.errors.noCategory"));

      const lines: string[] = [];
      for (const row of rows) {
        if (row.kind === "exec") {
          const v = row.value.trim();
          if (v) lines.push(v);
          continue;
        }
        const id = row.choice === NEW ? row.newId.trim() : row.choice;
        if (!id) throw new Error(t("recordReviewDialog.errors.missingPasswordId", { label: row.label }));
        if (row.choice === NEW) {
          await PasswordService.Add(id, row.newDesc, row.secret);
        }
        lines.push(`EXPECT:${row.label.toLowerCase()}`);
        lines.push(`SENDPASS:${id}`);
      }
      if (lines.length === 0) throw new Error(t("recordReviewDialog.errors.noCommands"));

      const host: Host = {
        id: "",
        name: name.trim(),
        command: lines.length === 1 ? lines[0] : undefined,
        commands: lines.length > 1 ? lines : undefined,
      };
      await HostService.AddHost({ targetFile, categoryPath: category.path, host });

      onSaved?.();
      dialog.close();
    } catch (err) {
      error = String(err);
      busy = false;
    }
  }
</script>

<dialog bind:this={dialog} class="glass-panel record-dialog" onclose={onDone}>
  <h3>{t("recordReviewDialog.title")}</h3>
  <p class="muted">{t("recordReviewDialog.subtitle")}</p>
  <form onsubmit={submit}>
    <div class="field">
      <label for="rec-name">{t("recordReviewDialog.nameLabel")}</label>
      <input id="rec-name" type="text" bind:value={name} required />
    </div>

    {#if loading}
      <p class="muted">{t("recordReviewDialog.loading")}</p>
    {:else}
      {#if files}<TargetFilePicker {files} bind:value={targetFile} />{/if}
      {#if categoryOptions.length === 0}
        <p class="error-text">{t("recordReviewDialog.category.empty")}</p>
      {:else}
        <div class="field">
          <label for="rec-category">{t("recordReviewDialog.category.label")}</label>
          <select id="rec-category" bind:value={categoryChoice}>
            {#each categoryOptions as opt, i}
              <option value={String(i)}>{opt.label}</option>
            {/each}
          </select>
        </div>
      {/if}

      <div class="field">
        <p class="section-label">{t("recordReviewDialog.steps.sectionLabel")}</p>
        {#each rows as row, i}
          <div class="glass-panel step-box">
            <div class="row between step-head">
              <span class="mono muted step-kind"
                >{row.kind === "exec" ? t("recordReviewDialog.steps.kindExec") : t("recordReviewDialog.steps.kindSendpass")}</span
              >
              <button type="button" class="icon-btn danger" onclick={() => removeRow(i)} title={t("recordReviewDialog.steps.removeTitle")}>
                <Trash2 size={13} strokeWidth={2} />
              </button>
            </div>
            {#if row.kind === "exec"}
              <input type="text" class="mono" bind:value={row.value} />
            {:else}
              <p class="mono muted step-label">{row.label}</p>
              <div class="field">
                <label for={`rec-choice-${i}`}>{t("recordReviewDialog.password.label")}</label>
                <select id={`rec-choice-${i}`} bind:value={row.choice}>
                  <option value={NEW}>{t("recordReviewDialog.password.newOption")}</option>
                  {#each passwords as p (p.id)}
                    <option value={p.id}>{p.id} — {p.description}</option>
                  {/each}
                </select>
              </div>
              {#if row.choice === NEW}
                <div class="row">
                  <div class="field" style="flex: 1">
                    <label for={`rec-id-${i}`}>{t("recordReviewDialog.password.idLabel")}</label>
                    <input id={`rec-id-${i}`} type="text" class="mono" bind:value={row.newId} required />
                  </div>
                  <div class="field" style="flex: 2">
                    <label for={`rec-desc-${i}`}>{t("recordReviewDialog.password.descLabel")}</label>
                    <input id={`rec-desc-${i}`} type="text" bind:value={row.newDesc} />
                  </div>
                </div>
                <div class="field">
                  <label for={`rec-secret-${i}`}>{t("recordReviewDialog.password.valueLabel")}</label>
                  <div class="row">
                    <input
                      id={`rec-secret-${i}`}
                      type={row.reveal ? "text" : "password"}
                      class="mono"
                      bind:value={row.secret}
                      style="flex: 1"
                    />
                    <button
                      type="button"
                      class="icon-btn"
                      onclick={() => (row.reveal = !row.reveal)}
                      title={row.reveal ? t("recordReviewDialog.password.hide") : t("recordReviewDialog.password.show")}
                    >
                      {#if row.reveal}<EyeOff size={14} strokeWidth={2} />{:else}<Eye size={14} strokeWidth={2} />{/if}
                    </button>
                  </div>
                </div>
              {/if}
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    {#if error}<p class="error-text">{error}</p>{/if}
    <div class="row end">
      <button type="button" onclick={() => dialog.close()}>{t("recordReviewDialog.buttons.cancel")}</button>
      <button type="submit" class="primary" disabled={busy || loading || categoryOptions.length === 0}
        >{t("recordReviewDialog.buttons.create")}</button
      >
    </div>
  </form>
</dialog>

<style>
  .record-dialog {
    width: min(480px, 92vw);
  }
  .step-box {
    padding: 10px;
    margin-bottom: 12px;
  }
  .step-head {
    margin-bottom: 6px;
  }
  .step-kind {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .step-label {
    margin: 0 0 8px;
    word-break: break-word;
  }
</style>
