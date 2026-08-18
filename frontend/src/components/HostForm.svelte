<script lang="ts">
  import { HostService, PasswordService } from "../../bindings/go-ssh-ui";
  import type { Config, Host } from "../../bindings/go-ssh/config/models";
  import type { Files } from "../../bindings/go-ssh-ui/internal/configx/models";
  import type { PasswordEntry } from "../../bindings/go-ssh/password/models";
  import TargetFilePicker from "./TargetFilePicker.svelte";
  import { t } from "../lib/i18n/i18n.svelte";

  let {
    files,
    allHosts,
    onSaved,
  }: {
    files: Files;
    allHosts: { id: string; name: string }[];
    onSaved: (cfg: Config) => void;
  } = $props();

  let dialog: HTMLDialogElement;
  let mode: "add" | "edit" = $state("add");
  let categoryPath: string[] = $state([]);
  let sourceFile = $state("");
  let originalName = $state("");

  let id = $state("");
  let name = $state("");
  let description = $state("");
  let advanced = $state(false);

  // "Basit" (structured) connection fields - drive go-ssh-ui's own SSH engine
  // once it lands (Phase 2); ignored by the go-ssh CLI.
  let hostAddr = $state("");
  let port: number | "" = $state("");
  let user = $state("");
  let authMethod: "" | "password" | "key" | "agent" = $state("");
  let identityFile = $state("");
  let passwordId = $state("");
  let jumpVia: string[] = $state([]);

  // "Gelişmiş" (legacy) mode - the raw ssh command string(s) go-ssh's CLI
  // already understands, steps included (SEND:/SENDPASS:/WAIT:/EXPECT:/INTERACT).
  let commandMode: "single" | "multi" = $state("single");
  let command = $state("");
  let steps: string[] = $state([]);

  let passwords: PasswordEntry[] = $state([]);
  let error: string | null = $state(null);
  let busy = $state(false);

  async function loadPasswords() {
    try {
      passwords = ((await PasswordService.List()) ?? []).filter((e): e is PasswordEntry => e !== null);
    } catch {
      passwords = [];
    }
  }

  function resetForm() {
    id = "";
    name = "";
    description = "";
    advanced = false;
    hostAddr = "";
    port = "";
    user = "";
    authMethod = "";
    identityFile = "";
    passwordId = "";
    jumpVia = [];
    commandMode = "single";
    command = "";
    steps = [];
    error = null;
  }

  export function openAdd(path: string[]) {
    mode = "add";
    categoryPath = path;
    resetForm();
    loadPasswords();
    dialog.showModal();
  }

  export function openEdit(path: string[], host: Host) {
    mode = "edit";
    categoryPath = path;
    sourceFile = host.sourceFile ?? files.configPath;
    originalName = host.name;

    id = host.id ?? "";
    name = host.name;
    description = host.description ?? "";
    hostAddr = host.host ?? "";
    port = host.port ?? "";
    user = host.user ?? "";
    authMethod = (host.authMethod as typeof authMethod) ?? "";
    identityFile = host.identityFile ?? "";
    passwordId = host.passwordId ?? "";
    jumpVia = host.jumpVia ? [...host.jumpVia] : [];

    if (host.commands && host.commands.length > 0) {
      advanced = true;
      commandMode = "multi";
      steps = [...host.commands];
      command = "";
    } else if (host.command) {
      advanced = true;
      commandMode = "single";
      command = host.command;
      steps = [];
    } else {
      advanced = false;
      commandMode = "single";
      command = "";
      steps = [];
    }

    error = null;
    loadPasswords();
    dialog.showModal();
  }

  function addStep() {
    steps = [...steps, ""];
  }
  function removeStep(i: number) {
    steps = steps.filter((_, idx) => idx !== i);
  }

  let helperDialog: HTMLDialogElement;
  let helperType: "send" | "sendpass" | "wait" | "expect" | "interact" = $state("send");
  let helperValue = $state("");

  function openHelper() {
    helperType = "send";
    helperValue = "";
    helperDialog.showModal();
  }

  function insertHelperStep() {
    let token: string;
    switch (helperType) {
      case "send":
        token = `SEND:${helperValue}`;
        break;
      case "sendpass":
        token = `SENDPASS:${helperValue}`;
        break;
      case "wait":
        token = `WAIT:${helperValue}`;
        break;
      case "expect":
        token = `EXPECT:${helperValue}`;
        break;
      default:
        token = "INTERACT";
    }
    steps = [...steps, token];
    helperDialog.close();
  }

  async function submit(e: Event) {
    e.preventDefault();
    error = null;
    busy = true;
    try {
      const host: Host = {
        id,
        name,
        description,
        host: hostAddr || undefined,
        port: port === "" ? undefined : Number(port),
        user: user || undefined,
        authMethod: authMethod || undefined,
        identityFile: identityFile || undefined,
        passwordId: passwordId || undefined,
        jumpVia: jumpVia.length > 0 ? jumpVia : undefined,
        command: advanced && commandMode === "single" ? command : undefined,
        commands: advanced && commandMode === "multi" ? steps : undefined,
      };

      const cfg =
        mode === "add"
          ? await HostService.AddHost({ targetFile: sourceFile, categoryPath, host })
          : await HostService.EditHost({ sourceFile, categoryPath, originalName, host });
      if (cfg) onSaved(cfg);
      dialog.close();
    } catch (err) {
      error = String(err);
    } finally {
      busy = false;
    }
  }
</script>

<dialog bind:this={dialog} class="glass-panel host-dialog">
  <h3>{mode === "add" ? t("hostForm.title.add") : t("hostForm.title.edit")}</h3>
  <form onsubmit={submit}>
    <div class="field">
      <label for="host-name">{t("hostForm.fields.name")}</label>
      <input id="host-name" type="text" bind:value={name} required />
    </div>
    <div class="field">
      <label for="host-desc">{t("hostForm.fields.description")}</label>
      <input id="host-desc" type="text" bind:value={description} />
    </div>

    {#if mode === "add"}
      <TargetFilePicker {files} bind:value={sourceFile} />
    {:else}
      <p class="muted mono">{sourceFile}</p>
    {/if}

    <div class="row mode-toggle">
      <button type="button" class={!advanced ? "primary" : ""} onclick={() => (advanced = false)}>{t("hostForm.mode.simple")}</button>
      <button type="button" class={advanced ? "primary" : ""} onclick={() => (advanced = true)}>{t("hostForm.mode.advanced")}</button>
    </div>

    {#if !advanced}
      <div class="field">
        <label for="host-addr">{t("hostForm.simple.address")}</label>
        <input id="host-addr" type="text" bind:value={hostAddr} placeholder="prod.example.com" />
      </div>
      <div class="row">
        <div class="field" style="flex: 1">
          <label for="host-port">{t("hostForm.simple.port")}</label>
          <input id="host-port" type="number" bind:value={port} placeholder="22" />
        </div>
        <div class="field" style="flex: 2">
          <label for="host-user">{t("hostForm.simple.user")}</label>
          <input id="host-user" type="text" bind:value={user} />
        </div>
      </div>
      <div class="field">
        <label for="host-auth">{t("hostForm.simple.auth.label")}</label>
        <select id="host-auth" bind:value={authMethod}>
          <option value="">{t("hostForm.simple.auth.none")}</option>
          <option value="password">{t("hostForm.simple.auth.password")}</option>
          <option value="key">{t("hostForm.simple.auth.key")}</option>
          <option value="agent">{t("hostForm.simple.auth.agent")}</option>
        </select>
      </div>
      {#if authMethod === "password"}
        <div class="field">
          <label for="host-pwid">{t("hostForm.simple.password")}</label>
          <select id="host-pwid" bind:value={passwordId}>
            <option value="">{t("hostForm.selectPlaceholder")}</option>
            {#each passwords as p (p.id)}
              <option value={p.id}>{p.id} — {p.description}</option>
            {/each}
          </select>
        </div>
      {:else if authMethod === "key"}
        <div class="field">
          <label for="host-identity">{t("hostForm.simple.identityFile")}</label>
          <input id="host-identity" type="text" bind:value={identityFile} placeholder="~/.ssh/id_ed25519" />
        </div>
      {/if}
      <div class="field">
        <label for="host-jump">{t("hostForm.simple.jump.label")}</label>
        <select id="host-jump" multiple bind:value={jumpVia} size={Math.min(4, Math.max(2, allHosts.length))}>
          {#each allHosts.filter((h) => h.id !== id) as h (h.id)}
            <option value={h.id}>{h.name}</option>
          {/each}
        </select>
        <p class="muted" style="margin-top: 4px">{t("hostForm.simple.jump.hint")}</p>
      </div>
    {:else}
      <div class="row mode-toggle">
        <button type="button" class={commandMode === "single" ? "primary" : ""} onclick={() => (commandMode = "single")}>{t("hostForm.advanced.commandMode.single")}</button>
        <button type="button" class={commandMode === "multi" ? "primary" : ""} onclick={() => (commandMode = "multi")}>{t("hostForm.advanced.commandMode.multi")}</button>
      </div>
      {#if commandMode === "single"}
        <div class="field">
          <label for="host-command">{t("hostForm.advanced.command")}</label>
          <input id="host-command" type="text" class="mono" bind:value={command} placeholder="ssh user@host" />
        </div>
      {:else}
        <div class="field">
          <label for="host-steps">{t("hostForm.advanced.steps")}</label>
          <div id="host-steps">
            {#each steps as _step, i}
              <div class="row step-row">
                <input type="text" class="mono" bind:value={steps[i]} />
                <button type="button" class="ghost danger" onclick={() => removeStep(i)}>✕</button>
              </div>
            {/each}
          </div>
          <div class="row" style="margin-top: 6px">
            <button type="button" onclick={addStep}>{t("hostForm.advanced.addStep")}</button>
            <button type="button" onclick={openHelper}>{t("hostForm.advanced.openHelper")}</button>
          </div>
        </div>
      {/if}
    {/if}

    {#if error}<p class="error-text">{error}</p>{/if}
    <div class="row end">
      <button type="button" onclick={() => dialog.close()}>{t("hostForm.buttons.cancel")}</button>
      <button type="submit" class="primary" disabled={busy}>{t("hostForm.buttons.save")}</button>
    </div>
  </form>
</dialog>

<dialog bind:this={helperDialog} class="glass-panel">
  <h3>{t("hostForm.helper.title")}</h3>
  <div class="field">
    <label>
      <input type="radio" name="helper-type" value="send" bind:group={helperType} /> {t("hostForm.helper.types.send")}
    </label>
    <label>
      <input type="radio" name="helper-type" value="sendpass" bind:group={helperType} /> {t("hostForm.helper.types.sendpass")}
    </label>
    <label>
      <input type="radio" name="helper-type" value="wait" bind:group={helperType} /> {t("hostForm.helper.types.wait")}
    </label>
    <label>
      <input type="radio" name="helper-type" value="expect" bind:group={helperType} /> {t("hostForm.helper.types.expect")}
    </label>
    <label>
      <input type="radio" name="helper-type" value="interact" bind:group={helperType} /> {t("hostForm.helper.types.interact")}
    </label>
  </div>
  {#if helperType === "sendpass"}
    <div class="field">
      <label for="helper-pw">{t("hostForm.helper.password")}</label>
      <select id="helper-pw" bind:value={helperValue}>
        <option value="">{t("hostForm.selectPlaceholder")}</option>
        {#each passwords as p (p.id)}
          <option value={p.id}>{p.id} — {p.description}</option>
        {/each}
      </select>
    </div>
  {:else if helperType !== "interact"}
    <div class="field">
      <label for="helper-val">{t("hostForm.helper.value")}</label>
      <input id="helper-val" type="text" bind:value={helperValue} />
    </div>
  {/if}
  <div class="row end">
    <button type="button" onclick={() => helperDialog.close()}>{t("hostForm.buttons.cancel")}</button>
    <button type="button" class="primary" onclick={insertHelperStep}>{t("hostForm.helper.add")}</button>
  </div>
</dialog>

<style>
  .host-dialog {
    width: min(520px, 92vw);
  }
  .mode-toggle {
    margin: 12px 0;
  }
  .mode-toggle button {
    flex: 1;
  }
  .step-row {
    margin-bottom: 6px;
  }
  .step-row input {
    flex: 1;
  }
  select[multiple] {
    width: 100%;
  }
</style>
