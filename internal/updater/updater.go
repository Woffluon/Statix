package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultGitHubRepo = "Woffluon/Statix"
	DefaultUserAgent  = "Statix-Updater"
	DefaultTimeout    = 30 * time.Second
)

var (
	ErrNoMatchingAsset    = errors.New("updater: no matching binary asset found for platform")
	ErrNoMatchingChecksum = errors.New("updater: no matching checksum asset found for platform")
	ErrChecksumMismatch   = errors.New("updater: SHA-256 checksum mismatch")
	ErrNilReleaseInfo     = errors.New("updater: release info is nil")
	ErrUpdateInProgress   = errors.New("updater: an update is already in progress")
)

// ReleaseInfo holds metadata regarding the latest published release.
type ReleaseInfo struct {
	Version         string    `json:"version"`
	TagName         string    `json:"tag_name"`
	ReleaseNotes    string    `json:"release_notes"`
	PublishedAt     time.Time `json:"published_at"`
	AssetURL        string    `json:"asset_url"`
	ChecksumURL     string    `json:"checksum_url"`
	UpdateAvailable bool      `json:"update_available"`
}

// Semver represents parsed major.minor.patch version components.
type Semver struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
}

// ParseSemver parses a semver string like "v1.2.3", "1.2.3", "v1.2.3-beta.1".
func ParseSemver(v string) (Semver, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return Semver{}, errors.New("empty version string")
	}

	var prerelease string
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		prerelease = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Semver{}, fmt.Errorf("invalid semver format: %s", v)
	}

	var s Semver
	s.Prerelease = prerelease

	var err error
	s.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid major version: %w", err)
	}

	if len(parts) > 1 {
		s.Minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return Semver{}, fmt.Errorf("invalid minor version: %w", err)
		}
	}

	if len(parts) > 2 {
		s.Patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Semver{}, fmt.Errorf("invalid patch version: %w", err)
		}
	}

	return s, nil
}

// CompareVersions compares two semver strings.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	s1, err1 := ParseSemver(v1)
	s2, err2 := ParseSemver(v2)
	if err1 != nil || err2 != nil {
		// Fallback to lexicographic comparison if not valid semver
		return strings.Compare(v1, v2)
	}

	if s1.Major != s2.Major {
		if s1.Major < s2.Major {
			return -1
		}
		return 1
	}
	if s1.Minor != s2.Minor {
		if s1.Minor < s2.Minor {
			return -1
		}
		return 1
	}
	if s1.Patch != s2.Patch {
		if s1.Patch < s2.Patch {
			return -1
		}
		return 1
	}

	// When major.minor.patch are equal:
	// A release without a prerelease has higher precedence than one with a prerelease.
	if s1.Prerelease == "" && s2.Prerelease != "" {
		return 1
	}
	if s1.Prerelease != "" && s2.Prerelease == "" {
		return -1
	}
	return strings.Compare(s1.Prerelease, s2.Prerelease)
}

// IsNewerVersion returns true if latest is strictly newer than current.
// If current is "dev", returns true.
func IsNewerVersion(latest, current string) bool {
	cleanCurrent := strings.TrimSpace(strings.ToLower(current))
	if cleanCurrent == "dev" || cleanCurrent == "development" || cleanCurrent == "" {
		return true
	}
	return CompareVersions(latest, current) > 0
}

// ParseChecksum parses the hex hash from a sha256sum file content.
// Handles formats: "<hash>  <filename>", "<hash> *<filename>", or raw "<hash>".
func ParseChecksum(content []byte, expectedFilename string) (string, error) {
	lines := strings.Split(string(content), "\n")
	var fallbackHash string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		hashCandidate := strings.ToLower(fields[0])
		if len(hashCandidate) != 64 {
			continue
		}

		if _, err := hex.DecodeString(hashCandidate); err != nil {
			continue
		}

		// Keep first valid hash as fallback
		if fallbackHash == "" {
			fallbackHash = hashCandidate
		}

		// If filename is present on the line, verify match if specified
		if len(fields) > 1 && expectedFilename != "" {
			fn := strings.TrimPrefix(fields[1], "*")
			if strings.EqualFold(filepath.Base(fn), filepath.Base(expectedFilename)) {
				return hashCandidate, nil
			}
		}
	}

	if fallbackHash != "" {
		return fallbackHash, nil
	}

	// Fallback check: if the entire content is just a 64-char hex string
	trimmed := strings.ToLower(strings.TrimSpace(string(content)))
	if len(trimmed) == 64 {
		if _, err := hex.DecodeString(trimmed); err == nil {
			return trimmed, nil
		}
	}

	return "", fmt.Errorf("updater: could not find valid 64-character SHA-256 hash in checksum file")
}

// VerifySHA256 verifies that data matches the expected 64-char hex SHA-256 hash.
func VerifySHA256(data []byte, expectedHex string) error {
	expectedHex = strings.ToLower(strings.TrimSpace(expectedHex))
	hasher := sha256.New()
	hasher.Write(data)
	actualHex := hex.EncodeToString(hasher.Sum(nil))

	if !strings.EqualFold(actualHex, expectedHex) {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedHex, actualHex)
	}
	return nil
}

// GitHubRelease represents the GitHub latest release API response.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a single downloadable asset in a GitHub release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Updater configures and orchestrates update checks and application.
type Updater struct {
	Repo               string
	BaseURL            string
	Client             *http.Client
	GOOS               string
	GOARCH             string
	ExecutablePathFunc func() (string, error)
	RestartFunc        func()

	mu sync.Mutex
}

// New creates an Updater with sensible production defaults.
func New() *Updater {
	return &Updater{
		Repo:               DefaultGitHubRepo,
		BaseURL:            "https://api.github.com",
		Client:             &http.Client{Timeout: DefaultTimeout},
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		ExecutablePathFunc: os.Executable,
		RestartFunc: func() {
			os.Exit(0)
		},
	}
}

// DefaultUpdater is the package-level default updater instance.
var DefaultUpdater = New()

// CheckUpdate checks GitHub for the latest release and compares with currentVersion.
func CheckUpdate(ctx context.Context, currentVersion string) (*ReleaseInfo, error) {
	return DefaultUpdater.CheckUpdate(ctx, currentVersion)
}

// ApplyUpdate downloads, validates, installs the update, and schedules a restart.
func ApplyUpdate(ctx context.Context, info *ReleaseInfo, logger *slog.Logger) error {
	return DefaultUpdater.ApplyUpdate(ctx, info, logger)
}

// CheckUpdate checks GitHub for the latest release and compares with currentVersion.
func (u *Updater) CheckUpdate(ctx context.Context, currentVersion string) (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(u.BaseURL, "/"), u.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("updater: failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater: release check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("updater: GitHub API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var ghRel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&ghRel); err != nil {
		return nil, fmt.Errorf("updater: failed to parse release JSON: %w", err)
	}

	targetArch := u.GOARCH
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	// Assets are named statix-linux-${ARCH} and statix-linux-${ARCH}.sha256
	binaryName := fmt.Sprintf("statix-linux-%s", targetArch)
	checksumName := fmt.Sprintf("statix-linux-%s.sha256", targetArch)

	var binaryURL, checksumURL string
	for _, asset := range ghRel.Assets {
		if asset.Name == binaryName {
			binaryURL = asset.BrowserDownloadURL
		} else if asset.Name == checksumName {
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if binaryURL == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoMatchingAsset, binaryName)
	}
	if checksumURL == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoMatchingChecksum, checksumName)
	}

	version := strings.TrimPrefix(ghRel.TagName, "v")
	isAvailable := IsNewerVersion(ghRel.TagName, currentVersion)

	return &ReleaseInfo{
		Version:         version,
		TagName:         ghRel.TagName,
		ReleaseNotes:    ghRel.Body,
		PublishedAt:     ghRel.PublishedAt,
		AssetURL:        binaryURL,
		ChecksumURL:     checksumURL,
		UpdateAvailable: isAvailable,
	}, nil
}

// ApplyUpdate downloads the binary and checksum, verifies integrity, replaces running binary, and restarts.
func (u *Updater) ApplyUpdate(ctx context.Context, info *ReleaseInfo, logger *slog.Logger) error {
	if info == nil {
		return ErrNilReleaseInfo
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if logger == nil {
		logger = slog.Default()
	}

	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}

	// 1. Resolve current executable path
	execFunc := u.ExecutablePathFunc
	if execFunc == nil {
		execFunc = os.Executable
	}

	rawPath, err := execFunc()
	if err != nil {
		return fmt.Errorf("updater: failed to locate current executable: %w", err)
	}
	execPath, err := filepath.EvalSymlinks(rawPath)
	if err != nil {
		execPath = rawPath
	}

	logger.Info("updater: starting update application",
		"target_version", info.Version,
		"exec_path", execPath,
	)

	// 2. Download checksum file
	logger.Info("updater: fetching checksum", "url", info.ChecksumURL)
	chkReq, err := http.NewRequestWithContext(ctx, http.MethodGet, info.ChecksumURL, nil)
	if err != nil {
		return fmt.Errorf("updater: checksum request creation failed: %w", err)
	}
	chkReq.Header.Set("User-Agent", DefaultUserAgent)

	chkResp, err := client.Do(chkReq)
	if err != nil {
		return fmt.Errorf("updater: checksum download failed: %w", err)
	}
	defer chkResp.Body.Close()

	if chkResp.StatusCode != http.StatusOK {
		return fmt.Errorf("updater: checksum download returned HTTP %d", chkResp.StatusCode)
	}

	chkBytes, err := io.ReadAll(io.LimitReader(chkResp.Body, 1024*64))
	if err != nil {
		return fmt.Errorf("updater: failed to read checksum bytes: %w", err)
	}

	expectedHash, err := ParseChecksum(chkBytes, filepath.Base(info.AssetURL))
	if err != nil {
		return fmt.Errorf("updater: invalid checksum format: %w", err)
	}
	logger.Info("updater: expected SHA-256 hash retrieved", "hash", expectedHash)

	// 3. Download binary to a temporary staging file
	stagingDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(stagingDir, "statix-update-*")
	if err != nil {
		// Fallback to system temp directory if stagingDir is not writable
		stagingDir = os.TempDir()
		tmpFile, err = os.CreateTemp(stagingDir, "statix-update-*")
		if err != nil {
			return fmt.Errorf("updater: failed to create temporary update file: %w", err)
		}
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) // Cleanup if still present
	}()

	logger.Info("updater: downloading binary", "url", info.AssetURL, "staging_path", tmpPath)
	binReq, err := http.NewRequestWithContext(ctx, http.MethodGet, info.AssetURL, nil)
	if err != nil {
		return fmt.Errorf("updater: binary request creation failed: %w", err)
	}
	binReq.Header.Set("User-Agent", DefaultUserAgent)

	binResp, err := client.Do(binReq)
	if err != nil {
		return fmt.Errorf("updater: binary download failed: %w", err)
	}
	defer binResp.Body.Close()

	if binResp.StatusCode != http.StatusOK {
		return fmt.Errorf("updater: binary download returned HTTP %d", binResp.StatusCode)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	// Limit download size to 256MB for safety
	written, err := io.Copy(writer, io.LimitReader(binResp.Body, 256*1024*1024))
	if err != nil {
		return fmt.Errorf("updater: failed writing binary to disk: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("updater: failed to sync binary to disk: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("updater: failed to close temporary file: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	logger.Info("updater: binary downloaded", "bytes", written, "actual_hash", actualHash)

	// 4. Verify SHA-256 hash
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedHash, actualHash)
	}
	logger.Info("updater: SHA-256 hash verified successfully")

	// 5. Set executable permissions (0755)
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("updater: failed to chmod 0755: %w", err)
	}

	// 6. Atomically replace the running binary
	if err := replaceBinary(tmpPath, execPath); err != nil {
		return fmt.Errorf("updater: failed to replace executable: %w", err)
	}

	logger.Info("updater: executable binary replaced successfully", "path", execPath)

	// 7. Schedule graceful restart
	restart := u.RestartFunc
	if restart != nil {
		logger.Info("updater: scheduling process restart...")
		go func() {
			time.Sleep(1 * time.Second)
			restart()
		}()
	}

	return nil
}

// replaceBinary handles atomic replacement of the running executable.
// Handles POSIX unlinking / renaming and fallback when text-file is busy.
func replaceBinary(src, dst string) error {
	// Try direct rename first (atomic on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If direct rename failed (e.g. ETXTBSY on Linux or cross-device link),
	// attempt to move dst to dst.old first, then rename src to dst.
	backupPath := dst + ".old." + strconv.FormatInt(time.Now().UnixNano(), 10)
	_ = os.Remove(backupPath)

	if err := os.Rename(dst, backupPath); err != nil {
		// If dst cannot be moved, try removing dst directly
		if rmErr := os.Remove(dst); rmErr != nil {
			return fmt.Errorf("could not unlink running executable (%v), rename error: %w", rmErr, err)
		}
	} else {
		defer os.Remove(backupPath)
	}

	// Now move new binary to dst
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// If rename still failed (e.g. cross-device EXDEV), copy bytes
	return copyFileAndChmod(src, dst, 0755)
}

func copyFileAndChmod(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
