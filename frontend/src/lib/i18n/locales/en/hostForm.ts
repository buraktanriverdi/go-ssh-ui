export const hostForm = {
  title: {
    add: "Add Host",
    edit: "Edit Host",
  },
  fields: {
    name: "Name",
    description: "Description",
  },
  mode: {
    simple: "Simple",
    advanced: "Advanced (raw command)",
  },
  simple: {
    address: "Address",
    port: "Port",
    user: "User",
    auth: {
      label: "Authentication",
      none: "Not selected",
      password: "Saved password",
      key: "SSH key",
      agent: "ssh-agent",
    },
    password: "Password",
    identityFile: "Key file",
    jump: {
      label: "Jump host chain (in order)",
      hint: "Hold Ctrl/Cmd to select multiple hosts.",
    },
  },
  advanced: {
    commandMode: {
      single: "Single command",
      multi: "Multi-step",
    },
    command: "Command",
    steps: "Steps",
    addStep: "+ Add step",
    openHelper: "Add… (SEND/SENDPASS/WAIT/EXPECT)",
  },
  helper: {
    title: "Add Step",
    types: {
      send: "Send text (SEND:)",
      sendpass: "Send saved password (SENDPASS:)",
      wait: "Wait N seconds (WAIT:)",
      expect: "Wait for text (EXPECT:)",
      interact: "Leave control to the user (INTERACT)",
    },
    password: "Password",
    value: "Value",
    add: "Add",
  },
  selectPlaceholder: "Select…",
  buttons: {
    cancel: "Cancel",
    save: "Save",
  },
};
