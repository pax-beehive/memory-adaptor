package runtime

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

func NewActiveSessionID(kind string) string {
	return "paxm-" + normalizedSessionKind(kind) + "-" + strings.ToLower(rand.Text())
}

func (r *Runtime) ActiveCLISessionID() string {
	workspace, err := os.Getwd()
	if err != nil {
		workspace = "unknown"
	}
	return stableActiveSessionID("cli", r.Config.Identity.UserID, r.ConfigPath, workspace)
}

func stableActiveSessionID(kind string, parts ...string) string {
	identity := strings.Join(append([]string{normalizedSessionKind(kind)}, parts...), "\x00")
	sum := sha256.Sum256([]byte(identity))
	return "paxm-" + normalizedSessionKind(kind) + "-" + hex.EncodeToString(sum[:12])
}

func normalizedSessionKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return "runtime"
	}
	return kind
}
