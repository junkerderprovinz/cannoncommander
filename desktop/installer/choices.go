package main

import "github.com/junkerderprovinz/pommelporter"

// fileManager is the installer's offer of the switch in the app's settings:
// the program makes itself the computer's file manager, and before an
// uninstall it gives the folders back, whoever switched it on. The labels are
// the app's own, desktop.fmLabel in its languages.
var fileManager = pommelporter.Choice{
	ID: "fileManager",
	Label: map[string]string{
		"ar": "تعيينه مدير الملفات الافتراضي",
		"bg": "Задай като файлов мениджър по подразбиране",
		"ca": "Defineix-lo com a gestor de fitxers predeterminat",
		"cs": "Nastavit jako výchozí správce souborů",
		"da": "Gør til standard-filhåndtering",
		"de": "Als Standard-Dateimanager festlegen",
		"el": "Ορισμός ως προεπιλεγμένη διαχείριση αρχείων",
		"en": "Make it the default file manager",
		"es": "Usar como gestor de archivos predeterminado",
		"et": "Määra vaikimisi failihalduriks",
		"eu": "Ezarri fitxategi-kudeatzaile lehenetsi gisa",
		"fa": "تنظیم به‌عنوان مدیر فایل پیش‌فرض",
		"fi": "Aseta oletustiedostonhallinnaksi",
		"fr": "Définir comme gestionnaire de fichiers par défaut",
		"gl": "Usar como xestor de ficheiros predeterminado",
		"he": "הגדרה כמנהל הקבצים ברירת המחדל",
		"hi": "डिफ़ॉल्ट फ़ाइल मैनेजर बनाएँ",
		"hr": "Postavi kao zadani upravitelj datoteka",
		"hu": "Legyen az alapértelmezett fájlkezelő",
		"id": "Jadikan pengelola berkas bawaan",
		"is": "Gera að sjálfgefnum skráastjóra",
		"it": "Imposta come gestore file predefinito",
		"ja": "既定のファイルマネージャーにする",
		"ko": "기본 파일 관리자로 설정",
		"lt": "Nustatyti kaip numatytąją failų tvarkyklę",
		"lv": "Iestatīt kā noklusējuma failu pārvaldnieku",
		"ms": "Jadikan pengurus fail lalai",
		"nl": "Instellen als standaard bestandsbeheerder",
		"no": "Gjør til standard filbehandler",
		"pl": "Ustaw jako domyślny menedżer plików",
		"pt": "Definir como gestor de ficheiros predefinido",
		"ro": "Setează ca manager de fișiere implicit",
		"ru": "Сделать файловым менеджером по умолчанию",
		"sk": "Nastaviť ako predvoleného správcu súborov",
		"sl": "Nastavi kot privzeti upravitelj datotek",
		"sr": "Постави као подразумевани управљач датотекама",
		"sv": "Gör till standardfilhanterare",
		"th": "ตั้งเป็นตัวจัดการไฟล์เริ่มต้น",
		"tr": "Varsayılan dosya yöneticisi yap",
		"uk": "Зробити типовим файловим менеджером",
		"vi": "Đặt làm trình quản lý tệp mặc định",
		"zh": "设为默认文件管理器",
	},
	Platforms: []string{"windows", "linux"},
	Do:        []string{"--file-manager", "on"},
	Undo:      []string{"--file-manager", "off"},
}
