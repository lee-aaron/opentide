package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

func cmdAdmin() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli admin <secret>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "secret":
		cmdAdminSecret()
	default:
		fmt.Fprintf(os.Stderr, "Unknown admin command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdAdminSecret() {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate random bytes: %v\n", err)
		os.Exit(1)
	}

	secret := hex.EncodeToString(b)
	fmt.Println(secret)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Set this as your admin secret:")
	fmt.Fprintf(os.Stderr, "  export OPENTIDE_ADMIN_SECRET=%s\n", secret)
}
