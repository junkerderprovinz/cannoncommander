package main

// product names the installation for all users: its entry under Apps and its
// folder under ProgramData, and the user's own folder for the data. appID
// names the single-instance lock. Both are name and id in installer/main.go.
var (
	product = "CannonCommander"
	appID   = "com.junkerderprovinz.cannoncommander"
)

// machineSettingsFile holds the installed copy's Update automatically switch
// in the folder under ProgramData, which the installer opens to every user.
const machineSettingsFile = "settings.json"
