<script lang="ts">
  import { HostService } from "../../bindings/go-ssh-ui";
  import type { Config, Category } from "../../bindings/go-ssh/config/models";
  import type { Files } from "../../bindings/go-ssh-ui/internal/configx/models";
  import TargetFilePicker from "./TargetFilePicker.svelte";
  import { t } from "../lib/i18n/i18n.svelte";

  let { files, onSaved }: { files: Files; onSaved: (cfg: Config) => void } = $props();

  let dialog: HTMLDialogElement;
  let mode: "add" | "edit" = $state("add");
  let parentPath: string[] = $state([]);
  let categoryPath: string[] = $state([]);
  let sourceFile = $state("");
  let name = $state("");
  let description = $state("");
  let error: string | null = $state(null);
  let busy = $state(false);

  export function openAdd(parent: string[]) {
    mode = "add";
    parentPath = parent;
    name = "";
    description = "";
    error = null;
    dialog.showModal();
  }

  export function openEdit(path: string[], category: Category) {
    mode = "edit";
    categoryPath = path;
    sourceFile = category.sourceFile ?? files.configPath;
    name = category.name;
    description = category.description ?? "";
    error = null;
    dialog.showModal();
  }

  async function submit(e: Event) {
    e.preventDefault();
    error = null;
    busy = true;
    try {
      const cfg =
        mode === "add"
          ? await HostService.AddCategory({ targetFile: sourceFile, parentPath, name, description })
          : await HostService.EditCategory({ sourceFile, categoryPath, name, description });
      if (cfg) onSaved(cfg);
      dialog.close();
    } catch (err) {
      error = String(err);
    } finally {
      busy = false;
    }
  }
</script>

<dialog bind:this={dialog} class="glass-panel">
  <h3>{mode === "add" ? t("categoryForm.addTitle") : t("categoryForm.editTitle")}</h3>
  <form onsubmit={submit}>
    <div class="field">
      <label for="cat-name">{t("categoryForm.nameLabel")}</label>
      <input id="cat-name" type="text" bind:value={name} required />
    </div>
    <div class="field">
      <label for="cat-desc">{t("categoryForm.descriptionLabel")}</label>
      <input id="cat-desc" type="text" bind:value={description} />
    </div>
    {#if mode === "add"}
      <TargetFilePicker {files} bind:value={sourceFile} />
    {:else}
      <p class="muted mono">{sourceFile}</p>
    {/if}
    {#if error}<p class="error-text">{error}</p>{/if}
    <div class="row end">
      <button type="button" onclick={() => dialog.close()}>{t("categoryForm.cancel")}</button>
      <button type="submit" class="primary" disabled={busy}>{t("categoryForm.save")}</button>
    </div>
  </form>
</dialog>
