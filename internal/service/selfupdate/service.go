// Package selfupdate 从官方发布资产更新当前 Skills Seed 可执行文件。
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/metadata"
)

const (
	// defaultAPIBaseURL 是官方发布元数据的 GitHub API 地址。
	defaultAPIBaseURL = "https://api.github.com/repos/silaswei-io/skills-seed"
	// checksumAssetName 是每个发布必须提供的 SHA-256 校验文件名。
	checksumAssetName = "checksums.txt"
	// latestVersion 是 GitHub Releases 最新稳定版本的选择标记。
	latestVersion = "latest"
)

var (
	// ErrUnsupportedPlatform 表示当前发布流程没有对应平台资产。
	ErrUnsupportedPlatform = errors.New("unsupported update platform")
	// ErrReleaseAssetMissing 表示目标发布缺少必需资产。
	ErrReleaseAssetMissing = errors.New("release asset is missing")
	// ErrChecksumMismatch 表示下载资产未通过发布校验和验证。
	ErrChecksumMismatch = errors.New("release asset checksum mismatch")
)

// Result 描述一次成功更新的目标版本和二进制位置。
type Result struct {
	Version         string
	ExecutablePath  string
	RestartRequired bool
}

// Service 下载、校验并替换当前可执行文件。
type Service struct {
	apiBaseURL string
	httpClient *http.Client
	executable func() (string, error)
	goos       string
	goarch     string
}

// New 创建使用官方 GitHub Releases API 的更新服务。
func New() *Service {
	return &Service{
		apiBaseURL: defaultAPIBaseURL,
		httpClient: http.DefaultClient,
		executable: os.Executable,
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
	}
}

// Update 将当前 CLI 更新到 latest 或指定发布版本。
func (s *Service) Update(ctx context.Context, version string) (Result, error) {
	if s == nil {
		return Result{}, errors.New("update service is unavailable")
	}
	if _, err := releasePlatform(s.goos, s.goarch); err != nil {
		return Result{}, err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = latestVersion
	}

	release, err := s.loadRelease(ctx, version)
	if err != nil {
		return Result{}, err
	}
	assetName, binaryName, err := releaseAssetName(release.TagName, s.goos, s.goarch)
	if err != nil {
		return Result{}, err
	}
	asset, ok := release.asset(assetName)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrReleaseAssetMissing, assetName)
	}
	checksums, ok := release.asset(checksumAssetName)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrReleaseAssetMissing, checksumAssetName)
	}

	executablePath, err := s.executablePath()
	if err != nil {
		return Result{}, err
	}
	restartRequired, err := s.install(ctx, executablePath, asset, checksums, assetName, binaryName)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Version:         release.TagName,
		ExecutablePath:  executablePath,
		RestartRequired: restartRequired,
	}, nil
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func (r release) asset(name string) (releaseAsset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name && strings.TrimSpace(asset.URL) != "" {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func (s *Service) loadRelease(ctx context.Context, version string) (release, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(s.apiBaseURL), "/")
	if baseURL == "" {
		return release{}, errors.New("release API URL is empty")
	}
	path := "/releases/latest"
	if version != latestVersion {
		path = "/releases/tags/" + url.PathEscape(version)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return release{}, fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "skills-seed/"+metadata.ProgramVersion)
	response, err := s.client().Do(request)
	if err != nil {
		return release{}, fmt.Errorf("load release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return release{}, fmt.Errorf("load release: unexpected HTTP status %s", response.Status)
	}

	var value release
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return release{}, fmt.Errorf("decode release metadata: %w", err)
	}
	value.TagName = strings.TrimSpace(value.TagName)
	if value.TagName == "" {
		return release{}, errors.New("release metadata has no tag name")
	}
	return value, nil
}

func (s *Service) install(ctx context.Context, executablePath string, asset, checksums releaseAsset, assetName, binaryName string) (bool, error) {
	executableInfo, err := os.Stat(executablePath)
	if err != nil {
		return false, fmt.Errorf("inspect current executable: %w", err)
	}
	directory := filepath.Dir(executablePath)
	archiveFile, err := os.CreateTemp(directory, ".skills-seed-update-*.archive")
	if err != nil {
		return false, fmt.Errorf("create update archive: %w", err)
	}
	archivePath := archiveFile.Name()
	archiveFile.Close()
	defer os.Remove(archivePath)

	digest, err := s.downloadTo(ctx, asset.URL, archivePath)
	if err != nil {
		return false, fmt.Errorf("download %s: %w", assetName, err)
	}
	checksumData, err := s.download(ctx, checksums.URL)
	if err != nil {
		return false, fmt.Errorf("download %s: %w", checksumAssetName, err)
	}
	expectedDigest, err := checksumForAsset(checksumData, assetName)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(digest, expectedDigest) {
		return false, fmt.Errorf("%w: %s", ErrChecksumMismatch, assetName)
	}

	candidate, err := os.CreateTemp(directory, ".skills-seed-update-*")
	if err != nil {
		return false, fmt.Errorf("create update candidate: %w", err)
	}
	candidatePath := candidate.Name()
	if err := candidate.Close(); err != nil {
		os.Remove(candidatePath)
		return false, fmt.Errorf("close update candidate: %w", err)
	}
	cleanupCandidate := true
	defer func() {
		if cleanupCandidate {
			_ = os.Remove(candidatePath)
		}
	}()

	if err := extractBinary(archivePath, assetName, binaryName, candidatePath, executableInfo.Mode().Perm()); err != nil {
		return false, err
	}
	replacementDeferred, err := replaceExecutable(executablePath, candidatePath)
	if err != nil {
		return false, fmt.Errorf("replace current executable: %w", err)
	}
	if replacementDeferred {
		cleanupCandidate = false
	}
	return replacementDeferred, nil
}

func (s *Service) executablePath() (string, error) {
	path, err := s.executable()
	if err != nil {
		return "", fmt.Errorf("locate current executable: %w", err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("current executable path is empty")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	return resolved, nil
}

func (s *Service) client() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return http.DefaultClient
}

func (s *Service) download(ctx context.Context, assetURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

func (s *Service) downloadTo(ctx context.Context, assetURL, path string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", err
	}
	response, err := s.client().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected HTTP status %s", response.Status)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, hash), response.Body); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func releaseAssetName(version, goos, goarch string) (string, string, error) {
	platform, err := releasePlatform(goos, goarch)
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("skills-seed_%s_%s_%s%s", version, goos, platform.arch, platform.extension), platform.binaryName, nil
}

type releasePlatformInfo struct {
	arch       string
	extension  string
	binaryName string
}

func releasePlatform(goos, goarch string) (releasePlatformInfo, error) {
	arch, ok := releaseArch(goarch)
	if !ok {
		return releasePlatformInfo{}, fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}
	platform := releasePlatformInfo{arch: arch}
	switch goos {
	case "darwin", "linux":
		platform.extension = ".tar.gz"
		platform.binaryName = "skills-seed"
	case "windows":
		platform.extension = ".zip"
		platform.binaryName = "skills-seed.exe"
	default:
		return releasePlatformInfo{}, fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}
	return platform, nil
}

func releaseArch(goarch string) (string, bool) {
	switch goarch {
	case "amd64":
		return "x86_64", true
	case "arm64":
		return "arm64", true
	default:
		return "", false
	}
}

func checksumForAsset(data []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || filepath.Base(fields[len(fields)-1]) != assetName {
			continue
		}
		digest := strings.ToLower(strings.TrimSpace(fields[0]))
		if len(digest) != sha256.Size*2 {
			break
		}
		if _, err := hex.DecodeString(digest); err == nil {
			return digest, nil
		}
		break
	}
	return "", fmt.Errorf("%w: %s", ErrReleaseAssetMissing, assetName)
}

func extractBinary(archivePath, assetName, binaryName, targetPath string, mode os.FileMode) error {
	root := strings.TrimSuffix(strings.TrimSuffix(assetName, ".tar.gz"), ".zip")
	expectedPath := root + "/" + binaryName
	if strings.HasSuffix(assetName, ".tar.gz") {
		return extractTarGzBinary(archivePath, expectedPath, targetPath, mode)
	}
	if strings.HasSuffix(assetName, ".zip") {
		return extractZipBinary(archivePath, expectedPath, targetPath, mode)
	}
	return fmt.Errorf("%w: archive format for %s", ErrReleaseAssetMissing, assetName)
}

func extractTarGzBinary(archivePath, expectedPath, targetPath string, mode os.FileMode) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer reader.Close()
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Name != expectedPath {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("%w: %s is not a regular file", ErrReleaseAssetMissing, expectedPath)
		}
		return writeExtractedBinary(targetPath, mode, archive)
	}
	return fmt.Errorf("%w: %s", ErrReleaseAssetMissing, expectedPath)
}

func extractZipBinary(archivePath, expectedPath, targetPath string, mode os.FileMode) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name != expectedPath {
			continue
		}
		if entry.FileInfo().IsDir() {
			return fmt.Errorf("%w: %s is a directory", ErrReleaseAssetMissing, expectedPath)
		}
		reader, err := entry.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		return writeExtractedBinary(targetPath, mode, reader)
	}
	return fmt.Errorf("%w: %s", ErrReleaseAssetMissing, expectedPath)
}

func writeExtractedBinary(path string, mode os.FileMode, reader io.Reader) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func replaceExecutable(target, candidate string) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, os.Rename(candidate, target)
	}
	if err := scheduleWindowsReplacement(target, candidate); err != nil {
		return false, err
	}
	return true, nil
}

func scheduleWindowsReplacement(target, candidate string) error {
	script, err := os.CreateTemp(filepath.Dir(target), ".skills-seed-update-*.cmd")
	if err != nil {
		return err
	}
	scriptPath := script.Name()
	content := "@echo off\r\nping 127.0.0.1 -n 2 > nul\r\nmove /Y \"" + candidate + "\" \"" + target + "\" > nul\r\ndel \"%~f0\"\r\n"
	if _, err := io.WriteString(script, content); err != nil {
		script.Close()
		os.Remove(scriptPath)
		return err
	}
	if err := script.Close(); err != nil {
		os.Remove(scriptPath)
		return err
	}
	command := exec.Command("cmd.exe", "/C", scriptPath)
	if err := command.Start(); err != nil {
		os.Remove(scriptPath)
		return err
	}
	return nil
}
