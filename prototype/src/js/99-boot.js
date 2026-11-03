if (FS.nodeAt(CC.prefs.startFolder)?.dir) Files.panels[0].tabs[0] = newTab(CC.prefs.startFolder);
matchMedia('(prefers-color-scheme: light)').addEventListener('change', () => CC.prefs.theme === 'system' && CC.applyPrefs());
window.PW_GO = (to) => CC.go(to.view, to.tab);
CC.applyPrefs();
CC.render();
