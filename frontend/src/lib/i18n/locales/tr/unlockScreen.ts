export const unlockScreen = {
  loading: "Yükleniyor…",
  createIntro:
    "Henüz bir şifre deposu yok. Bir ana şifre oluştur - bu şifre ~/.go-ssh/passwords.enc dosyasını korur ve go-ssh CLI ile de paylaşılır.",
  unlockIntro: "Devam etmek için ana şifreni gir.",
  createPasswordLabel: "Ana şifre (en az 8 karakter)",
  confirmPasswordLabel: "Ana şifre (tekrar)",
  unlockPasswordLabel: "Ana şifre",
  createSubmit: "Oluştur ve Aç",
  unlockSubmit: "Kilidi Aç",
  errors: {
    tooShort: "Ana şifre en az 8 karakter olmalı.",
    mismatch: "Şifreler eşleşmiyor.",
    wrongPassword: "Ana şifre yanlış.",
  },
};
