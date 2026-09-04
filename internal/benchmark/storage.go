package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const privateSeedDirectory = "benchmark-private"

// WritePrivateSeed stores one raw seed in controller-only storage. The caller
// selects a path outside the repository; the function refuses symlinks and
// overwrites and creates files with owner-only permissions where supported.
func WritePrivateSeed(root, instanceID string, seed []byte) (string, error) {
	if err := validatePrivateInstanceID(instanceID); err != nil {
		return "", err
	}
	if len(seed) < 16 {
		return "", fmt.Errorf("seed must contain at least 16 bytes")
	}
	if err := ensurePrivateDirectory(root); err != nil {
		return "", err
	}
	path := filepath.Join(root, instanceID+".seed")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create private seed: %w", err)
	}
	if _, err := file.Write(seed); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write private seed: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close private seed: %w", err)
	}
	return path, nil
}

// BuildCommitmentManifest returns the only benchmark artifact suitable for
// public storage before evaluation.
func BuildCommitmentManifest(protocolDigest string, instances []GeneratedInstance) (CommitmentManifest, error) {
	manifest := CommitmentManifest{Version: CommitmentVersion, ProtocolDigest: protocolDigest, Commitments: make([]PublicCommitment, 0, len(instances))}
	for _, instance := range instances {
		if err := instance.Scenario.Validate(); err != nil {
			return CommitmentManifest{}, err
		}
		manifest.Commitments = append(manifest.Commitments, instance.Commitment)
	}
	manifest = manifest.Canonicalized()
	if err := manifest.Validate(); err != nil {
		return CommitmentManifest{}, err
	}
	return manifest, nil
}

// WritePublicCommitmentManifest writes a canonical public manifest without
// permitting replacement of an existing pre-evaluation commitment.
func WritePublicCommitmentManifest(path string, manifest CommitmentManifest) error {
	manifest = manifest.Canonicalized()
	if err := manifest.Validate(); err != nil {
		return err
	}
	body, err := CanonicalJSON(manifest)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create public commitment manifest: %w", err)
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return fmt.Errorf("write public commitment manifest: %w", err)
	}
	return file.Close()
}

func ensurePrivateDirectory(path string) error {
	if path == "" {
		return fmt.Errorf("private seed directory is required")
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private seed directory must not be a symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect private seed directory: %w", err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create private seed directory: %w", err)
	}
	return os.Chmod(path, 0o700)
}

func validatePrivateInstanceID(value string) error {
	if err := validateIdentifier("instance_id", value); err != nil {
		return err
	}
	if strings.ContainsAny(value, `\\/:`) || value == "." || value == ".." {
		return fmt.Errorf("instance_id is unsafe for private seed storage")
	}
	return nil
}
