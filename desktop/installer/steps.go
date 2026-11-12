package main

import "github.com/junkerderprovinz/pommelporter"

// helperPrograms fetches the modules of the installed version, ffmpeg, 7-Zip
// and mutool, which the program installs beside itself with --modules and
// reports on as it goes. A step that fails leaves the app to fetch them on
// its next start, or the scheduled task for a copy for all users.
var helperPrograms = pommelporter.Step{
	ID: "modules",
	Label: map[string]string{
		"ar": "جارٍ تنزيل البرامج المساعدة",
		"bg": "Изтегляне на помощните програми",
		"ca": "S'estan baixant els programes auxiliars",
		"cs": "Stahují se pomocné programy",
		"da": "Henter hjælpeprogrammerne",
		"de": "Hilfsprogramme werden geladen",
		"el": "Λήψη των βοηθητικών προγραμμάτων",
		"en": "Fetching the helper programs",
		"es": "Descargando los programas auxiliares",
		"et": "Abiprogrammide allalaadimine",
		"eu": "Programa laguntzaileak deskargatzen",
		"fa": "در حال دانلود برنامه‌های کمکی",
		"fi": "Ladataan apuohjelmia",
		"fr": "Téléchargement des programmes d'aide",
		"gl": "Descargando os programas auxiliares",
		"he": "מוריד את תוכניות העזר",
		"hi": "सहायक प्रोग्राम डाउनलोड हो रहे हैं",
		"hr": "Preuzimanje pomoćnih programa",
		"hu": "Segédprogramok letöltése",
		"id": "Mengunduh program pembantu",
		"is": "Sæki hjálparforritin",
		"it": "Download dei programmi di supporto",
		"ja": "補助プログラムをダウンロードしています",
		"ko": "도우미 프로그램을 다운로드하는 중",
		"lt": "Atsisiunčiamos pagalbinės programos",
		"lv": "Lejupielādē palīgprogrammas",
		"ms": "Memuat turun program pembantu",
		"nl": "Hulpprogramma's downloaden",
		"no": "Laster ned hjelpeprogrammene",
		"pl": "Pobieranie programów pomocniczych",
		"pt": "A transferir os programas auxiliares",
		"ro": "Se descarcă programele ajutătoare",
		"ru": "Загрузка вспомогательных программ",
		"sk": "Sťahujú sa pomocné programy",
		"sl": "Prenašanje pomožnih programov",
		"sr": "Преузимање помоћних програма",
		"sv": "Hämtar hjälpprogrammen",
		"th": "กำลังดาวน์โหลดโปรแกรมช่วยเหลือ",
		"tr": "Yardımcı programlar indiriliyor",
		"uk": "Завантаження допоміжних програм",
		"vi": "Đang tải các chương trình hỗ trợ",
		"zh": "正在下载辅助程序",
	},
	Args: []string{"--modules"},
}
