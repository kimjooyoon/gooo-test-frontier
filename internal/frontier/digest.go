package frontier

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestFile(path string) (string, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return DigestBytes(raw), raw, nil
}

func DigestJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(raw), nil
}

func ValidateDigest(value string) error {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return fmt.Errorf("invalid digest %q", value)
	}
	for _, char := range value[len("sha256:"):] {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return fmt.Errorf("invalid digest %q", value)
		}
	}
	return nil
}
