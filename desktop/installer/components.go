package main

import "github.com/junkerderprovinz/pommelporter"

// helperPrograms are the modules of the installed version, ffmpeg, 7-Zip and
// mutool, which the program installs beside itself with --modules and
// reports on as it goes. One that fails leaves the app to fetch them on its
// next start, or the scheduled task for a copy for all users.
var helperPrograms = pommelporter.Component{
	ID: "modules",
	Name: map[string]string{
		"ar": "البرامج المساعدة",
		"bg": "Помощни програми",
		"ca": "Programes auxiliars",
		"cs": "Pomocné programy",
		"da": "Hjælpeprogrammer",
		"de": "Hilfsprogramme",
		"el": "Βοηθητικά προγράμματα",
		"en": "Helper programs",
		"es": "Programas auxiliares",
		"et": "Abiprogrammid",
		"eu": "Programa laguntzaileak",
		"fa": "برنامه‌های کمکی",
		"fi": "Apuohjelmat",
		"fr": "Programmes d'aide",
		"gl": "Programas auxiliares",
		"he": "תוכניות עזר",
		"hi": "सहायक प्रोग्राम",
		"hr": "Pomoćni programi",
		"hu": "Segédprogramok",
		"id": "Program pembantu",
		"is": "Hjálparforrit",
		"it": "Programmi di supporto",
		"ja": "補助プログラム",
		"ko": "도우미 프로그램",
		"lt": "Pagalbinės programos",
		"lv": "Palīgprogrammas",
		"ms": "Program pembantu",
		"nl": "Hulpprogramma's",
		"no": "Hjelpeprogrammer",
		"pl": "Programy pomocnicze",
		"pt": "Programas auxiliares",
		"ro": "Programe ajutătoare",
		"ru": "Вспомогательные программы",
		"sk": "Pomocné programy",
		"sl": "Pomožni programi",
		"sr": "Помоћни програми",
		"sv": "Hjälpprogram",
		"th": "โปรแกรมช่วยเหลือ",
		"tr": "Yardımcı programlar",
		"uk": "Допоміжні програми",
		"vi": "Chương trình hỗ trợ",
		"zh": "辅助程序",
	},
	Run: []string{"--modules"},
}

// helperFolders are what --modules leaves beside the program, so an
// uninstall takes them along.
var helperFolders = []string{"modules", "modules.next", "modules.old"}
