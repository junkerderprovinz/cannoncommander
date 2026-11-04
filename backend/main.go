package main

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/cmd"
)

func main() {
	jwt.DecodeStrict = true
	cmd.Start()
}
