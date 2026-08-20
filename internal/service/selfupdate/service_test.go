package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpdateDownloadsVerifiesAndReplacesExecutable(t *testing.T) {
	const version = "v1.2.3"
	assetName, binaryName, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := tarGzAsset(t, assetName, binaryName, []byte("new executable"))
	client, transport := releaseClient(version, assetName, archive, hexDigest(archive))

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(client, executable, "linux", "amd64")

	events := make([]ProgressEvent, 0, 4)
	result, err := service.UpdateWithProgress(context.Background(), "latest", func(event ProgressEvent) {
		events = append(events, event)
	})
	require.NoError(t, err)
	require.Equal(t, version, result.Version)
	resolvedExecutable, err := filepath.EvalSymlinks(executable)
	require.NoError(t, err)
	require.Equal(t, resolvedExecutable, result.ExecutablePath)
	require.False(t, result.RestartRequired)
	content, err := os.ReadFile(executable)
	require.NoError(t, err)
	require.Equal(t, []byte("new executable"), content)
	require.Equal(t, []string{"/releases/latest", "/assets/" + assetName, "/assets/checksums.txt"}, transport.requested)
	require.Equal(t, []Stage{StageResolveRelease, StageDownloadAsset, StageVerifyAsset, StageInstallAsset}, uniqueStages(events))
	require.NotEmpty(t, events)
	var downloadEvent ProgressEvent
	for _, event := range events {
		if event.Stage == StageDownloadAsset && event.Downloaded > 0 {
			downloadEvent = event
			break
		}
	}
	require.Greater(t, downloadEvent.Downloaded, int64(0))
	require.Greater(t, downloadEvent.Total, int64(0))
}

func uniqueStages(events []ProgressEvent) []Stage {
	stages := make([]Stage, 0, len(events))
	for _, event := range events {
		if len(stages) == 0 || stages[len(stages)-1] != event.Stage {
			stages = append(stages, event.Stage)
		}
	}
	return stages
}

func TestNewUsesBoundedHTTPClient(t *testing.T) {
	service := New()
	require.NotNil(t, service.httpClient)
	require.Equal(t, 2*time.Minute, service.httpClient.Timeout)
}

func TestUpdateExplicitVersionUsesTagEndpoint(t *testing.T) {
	const version = "v2.0.0"
	assetName, binaryName, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := tarGzAsset(t, assetName, binaryName, []byte("new executable"))
	client, transport := releaseClient(version, assetName, archive, hexDigest(archive))

	service := testService(client, writeExecutable(t, []byte("old executable")), "linux", "amd64")
	_, err = service.Update(context.Background(), version)
	require.NoError(t, err)
	require.Equal(t, "/releases/tags/"+version, transport.requested[0])
}

func TestUpdateChecksumMismatchDoesNotReplaceExecutable(t *testing.T) {
	const version = "v1.2.3"
	assetName, binaryName, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := tarGzAsset(t, assetName, binaryName, []byte("new executable"))
	client, _ := releaseClient(version, assetName, archive, strings.Repeat("0", sha256.Size*2))

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(client, executable, "linux", "amd64")

	_, err = service.Update(context.Background(), "latest")
	require.ErrorIs(t, err, ErrChecksumMismatch)
	content, readErr := os.ReadFile(executable)
	require.NoError(t, readErr)
	require.Equal(t, []byte("old executable"), content)
}

func TestUpdateMalformedArchiveDoesNotReplaceExecutable(t *testing.T) {
	const version = "v1.2.3"
	assetName, _, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := []byte("not a gzip archive")
	client, _ := releaseClient(version, assetName, archive, hexDigest(archive))

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(client, executable, "linux", "amd64")

	_, err = service.Update(context.Background(), "latest")
	require.Error(t, err)
	content, readErr := os.ReadFile(executable)
	require.NoError(t, readErr)
	require.Equal(t, []byte("old executable"), content)
}

func TestUpdateRejectsUnsupportedPlatformBeforeNetworkAccess(t *testing.T) {
	service := testService(http.DefaultClient, writeExecutable(t, []byte("old executable")), "plan9", "amd64")

	_, err := service.Update(context.Background(), "latest")
	require.ErrorIs(t, err, ErrUnsupportedPlatform)
}

func TestUpdateMissingAssetDoesNotReplaceExecutable(t *testing.T) {
	client, _ := releaseClientMissingAsset("v1.2.3")

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(client, executable, "linux", "amd64")

	_, err := service.Update(context.Background(), "latest")
	require.ErrorIs(t, err, ErrReleaseAssetMissing)
	content, readErr := os.ReadFile(executable)
	require.NoError(t, readErr)
	require.Equal(t, []byte("old executable"), content)
}

func TestReleaseAssetNameMatchesReleaseWorkflow(t *testing.T) {
	cases := []struct {
		name       string
		goos       string
		goarch     string
		assetName  string
		binaryName string
	}{
		{name: "darwin arm64", goos: "darwin", goarch: "arm64", assetName: "skills-seed_v1.2.3_darwin_arm64.tar.gz", binaryName: "skills-seed"},
		{name: "linux amd64", goos: "linux", goarch: "amd64", assetName: "skills-seed_v1.2.3_linux_x86_64.tar.gz", binaryName: "skills-seed"},
		{name: "windows arm64", goos: "windows", goarch: "arm64", assetName: "skills-seed_v1.2.3_windows_arm64.zip", binaryName: "skills-seed.exe"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assetName, binaryName, err := releaseAssetName("v1.2.3", testCase.goos, testCase.goarch)
			require.NoError(t, err)
			require.Equal(t, testCase.assetName, assetName)
			require.Equal(t, testCase.binaryName, binaryName)
		})
	}
}

func testService(client *http.Client, executable, goos, goarch string) *Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &Service{
		apiBaseURL: "https://example.invalid",
		httpClient: client,
		executable: func() (string, error) {
			return executable, nil
		},
		goos:   goos,
		goarch: goarch,
	}
}

func writeExecutable(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "skills-seed")
	require.NoError(t, os.WriteFile(path, content, 0o755))
	return path
}

func releaseClient(version, assetName string, archive []byte, checksum string) (*http.Client, *releaseTransport) {
	transport := &releaseTransport{
		version:   version,
		assetName: assetName,
		archive:   archive,
		checksum:  checksum,
	}
	return &http.Client{Transport: transport}, transport
}

func releaseClientMissingAsset(version string) (*http.Client, *releaseTransport) {
	transport := &releaseTransport{version: version}
	return &http.Client{Transport: transport}, transport
}

type releaseTransport struct {
	version   string
	assetName string
	archive   []byte
	checksum  string
	requested []string
}

func (t *releaseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.requested = append(t.requested, request.URL.Path)
	switch request.URL.Path {
	case "/releases/latest", "/releases/tags/" + t.version:
		if t.assetName == "" {
			return newHTTPResponse(http.StatusOK, []byte(fmt.Sprintf(`{"tag_name":%q,"assets":[]}`, t.version))), nil
		}
		body := fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
			t.version, t.assetName, "https://example.invalid/assets/"+t.assetName, "https://example.invalid/assets/checksums.txt")
		return newHTTPResponse(http.StatusOK, []byte(body)), nil
	case "/assets/" + t.assetName:
		return newHTTPResponse(http.StatusOK, t.archive), nil
	case "/assets/checksums.txt":
		body := fmt.Sprintf("%s  %s\n", t.checksum, t.assetName)
		return newHTTPResponse(http.StatusOK, []byte(body)), nil
	default:
		return newHTTPResponse(http.StatusNotFound, []byte("not found")), nil
	}
}

func newHTTPResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func hexDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func tarGzAsset(t *testing.T, assetName, binaryName string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gzipWriter)
	path := strings.TrimSuffix(assetName, ".tar.gz") + "/" + binaryName
	require.NoError(t, archive.WriteHeader(&tar.Header{Name: path, Mode: 0o755, Size: int64(len(content))}))
	_, err := archive.Write(content)
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	require.NoError(t, gzipWriter.Close())
	return buffer.Bytes()
}
