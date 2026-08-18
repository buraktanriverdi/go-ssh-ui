export const recordReviewDialog = {
  title: "Create Host from Recording",
  subtitle: "Review what was detected - nothing is saved until you confirm.",
  nameLabel: "Host name",
  loading: "Loading…",
  category: {
    label: "Category",
    empty: "No categories yet - first create one from the Hosts tab.",
  },
  steps: {
    sectionLabel: "Steps",
    kindExec: "Command",
    kindSendpass: "Password",
    removeTitle: "Remove",
  },
  password: {
    label: "Password",
    newOption: "Create new password",
    idLabel: "ID",
    descLabel: "Description",
    valueLabel: "Captured value",
    show: "Show",
    hide: "Hide",
  },
  errors: {
    noSteps: "There must be at least one step",
    noCategory: "Select a category",
    missingPasswordId: "Enter a password ID for \"{label}\"",
    noCommands: "There must be at least one command",
  },
  buttons: {
    cancel: "Cancel",
    create: "Create Host",
  },
};
