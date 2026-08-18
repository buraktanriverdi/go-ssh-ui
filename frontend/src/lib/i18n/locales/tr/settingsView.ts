export const settingsView = {
  title: "Ayarlar",
  intro:
    "Pencerenin saydamlığını, bulanıklığını ve renk tonunu buradan ayarlayabilirsin. Saydamlık/bulanıklık/renk anında uygulanır; native pencere bulanıklığı bir sonraki açılışta devreye girer.",
  appearance: {
    heading: "Görünüm",
    opacityLabel: "Arka plan saydamlığı",
    opacityHint: "Düşük değer pencere arkasının (ve terminal arkaplanının) daha fazla görünmesini sağlar.",
    blurLabel: "Panel bulanıklığı",
    blurHint:
      "0, diyalog ve panellerdeki ek bulanıklığı tamamen kapatır (en keskin görünüm). Pencerenin tüm arkaplanını etkileyen native bulanıklık ayrı bir ayar - aşağıda.",
  },
  terminal: {
    heading: "Terminal",
    fontSizeLabel: "Yazı boyutu",
    fontSizeHint:
      "Yeni açılan terminallerin başlangıç yazı boyutu. Açık olan terminalleri etkilemez - her terminal kendi boyutunu Cmd+=/Cmd+-/Cmd+0 ile veya sağ tık menüsünden ayrı ayrı değiştirir.",
  },
  language: {
    heading: "Dil",
    intro: "Uygulama arayüz dilini seç.",
    systemLabel: "Sistem",
    systemHint: "İşletim sisteminin dilini otomatik izler.",
    turkishLabel: "Türkçe",
    englishLabel: "English",
  },
  backdrop: {
    heading: "Pencere arkaplanı (native)",
    hintDiff:
      "Yukarıdaki \"Panel bulanıklığı\"ndan farklı: diyalog kartları değil, pencerenin arkasında görünen masaüstünü/diğer pencereleri etkileyen katman bu - hepsi üstteki saydamlık ayarına uyar.",
    hintRestart: "Değişikliğin görünmesi için uygulamayı kapatıp yeniden açman gerekiyor.",
    saved: "Kaydedildi - bir sonraki açılışta uygulanacak.",
    options: {
      translucent: { label: "Bulanık (vibrancy)", hint: "Klasik frosted-glass macOS görünümü." },
      liquidGlass: {
        label: "Liquid Glass",
        hint: "Apple'ın yeni camsı efekti (macOS 15+; eski sürümlerde otomatik Bulanık'a döner).",
      },
      transparent: { label: "Şeffaf (bulanıksız)", hint: "Aynı derecede saydam, ama keskin - hiç bulanıklık yok." },
    },
  },
  hotkey: {
    heading: "Hotkey Penceresi",
    intro:
      "iTerm'deki gibi: bir global kısayolla, uygulama arka plandayken bile açılıp kapanan, tek bir yerel terminal içeren yüzen bir pencere.",
    enabled: "Etkin",
    shortcutLabel: "Kısayol",
    shortcutUnset: "Ayarlanmadı",
    changeShortcut: "Kısayolu değiştir",
    capturing: "Tuşlara basın… (iptal: Esc)",
    unsetHint: "Etkin ama henüz bir kısayol seçilmedi - \"Kısayolu değiştir\"e tıklayıp bir tuş kombinasyonuna bas.",
    opacityLabel: "Pencere saydamlığı",
    colorLabel: "Pencere arka plan rengi",
    backdropLabel: "Pencere arkaplanı (native)",
    backdropHint: "Yukarıdaki renk/saydamlık bunun üzerine biner. Değişikliğin görünmesi için uygulamayı kapatıp yeniden açman gerekiyor.",
    testButton: "Pencereyi test et",
    saved: "Kaydedildi",
    liveHint: "Kısayol/renk/saydamlık anında uygulanır - pencere arkaplanı (native) hariç, yeniden başlatma gerekmez.",
    errors: {
      unsupportedKey: "Bu tuş sistem geneli kısayol olarak desteklenmiyor. Lütfen farklı bir tuş deneyin (harf, rakam veya fonksiyon tuşu önerilir).",
      needsModifier: "Bu kısayol için en az bir değiştirici tuş (Cmd/Ctrl/Option/Shift) gerekli.",
    },
  },
  accent: {
    heading: "Renk tonu",
    customTitle: "Özel renk seç",
    colors: {
      blue: "Mavi",
      purple: "Mor",
      pink: "Pembe",
      red: "Kırmızı",
      orange: "Turuncu",
      yellow: "Sarı",
      green: "Yeşil",
      graphite: "Grafit",
    },
  },
  resetButton: "Varsayılanlara dön",
};
