export const unlockScreen = {
  loading: "Loading…",
  createIntro:
    "There's no password store yet. Create a master password - it protects ~/.go-ssh/passwords.enc and is shared with the go-ssh CLI.",
  unlockIntro: "Enter your master password to continue.",
  createPasswordLabel: "Master password (at least 8 characters)",
  confirmPasswordLabel: "Master password (again)",
  unlockPasswordLabel: "Master password",
  createSubmit: "Create and Unlock",
  unlockSubmit: "Unlock",
  errors: {
    tooShort: "The master password must be at least 8 characters.",
    mismatch: "Passwords don't match.",
    wrongPassword: "Wrong master password.",
  },
};
