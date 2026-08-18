export const hostForm = {
  title: {
    add: "Host Ekle",
    edit: "Hostu Düzenle",
  },
  fields: {
    name: "Ad",
    description: "Açıklama",
  },
  mode: {
    simple: "Basit",
    advanced: "Gelişmiş (ham komut)",
  },
  simple: {
    address: "Adres",
    port: "Port",
    user: "Kullanıcı",
    auth: {
      label: "Kimlik doğrulama",
      none: "Seçilmedi",
      password: "Kayıtlı şifre",
      key: "SSH anahtarı",
      agent: "ssh-agent",
    },
    password: "Şifre",
    identityFile: "Anahtar dosyası",
    jump: {
      label: "Jump host zinciri (sırayla)",
      hint: "Ctrl/Cmd ile birden çok host seçilebilir.",
    },
  },
  advanced: {
    commandMode: {
      single: "Tek komut",
      multi: "Çok adımlı",
    },
    command: "Komut",
    steps: "Adımlar",
    addStep: "+ Adım ekle",
    openHelper: "Ekle… (SEND/SENDPASS/WAIT/EXPECT)",
  },
  helper: {
    title: "Adım Ekle",
    types: {
      send: "Metin gönder (SEND:)",
      sendpass: "Kayıtlı şifre gönder (SENDPASS:)",
      wait: "N saniye bekle (WAIT:)",
      expect: "Metin bekle (EXPECT:)",
      interact: "Kontrolü kullanıcıya bırak (INTERACT)",
    },
    password: "Şifre",
    value: "Değer",
    add: "Ekle",
  },
  selectPlaceholder: "Seç…",
  buttons: {
    cancel: "Vazgeç",
    save: "Kaydet",
  },
};
