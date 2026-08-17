package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateDownloadsVerifiesAndReplacesExecutable(t *testing.T) {
	const version = "v1.2.3"
	assetName, binaryName, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := tarGzAsset(t, assetName, binaryName, []byte("new executable"))
	server, requested := releaseServer(t, version, assetName, archive)
	defer server.Close()

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(server.URL, executable, "linux", "amd64")

	result, err := service.Update(context.Background(), "latest")
	require.NoError(t, err)
	require.Equal(t, version, result.Version)
	resolvedExecutable, err := filepath.EvalSymlinks(executable)
	require.NoError(t, err)
	require.Equal(t, resolvedExecutable, result.ExecutablePath)
	require.False(t, result.RestartRequired)
	content, err := os.ReadFile(executable)
	require.NoError(t, err)
	require.Equal(t, []byte("new executable"), content)
	require.Equal(t, []string{"/releases/latest", "/assets/" + assetName, "/assets/checksums.txt"}, *requested)
}

func TestUpdateExplicitVersionUsesTagEndpoint(t *testing.T) {
	const version = "v2.0.0"
	assetName, binaryName, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := tarGzAsset(t, assetName, binaryName, []byte("new executable"))
	server, requested := releaseServer(t, version, assetName, archive)
	defer server.Close()

	service := testService(server.URL, writeExecutable(t, []byte("old executable")), "linux", "amd64")
	_, err = service.Update(context.Background(), version)
	require.NoError(t, err)
	require.Equal(t, "/releases/tags/"+version, (*requested)[0])
}

func TestUpdateChecksumMismatchDoesNotReplaceExecutable(t *testing.T) {
	const version = "v1.2.3"
	assetName, binaryName, err := releaseAssetName(version, "linux", "amd64")
	require.NoError(t, err)
	archive := tarGzAsset(t, assetName, binaryName, []byte("new executable"))
	server, _ := releaseServerWithChecksum(t, version, assetName, archive, strings.Repeat("0", sha256.Size*2))
	defer server.Close()

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(server.URL, executable, "linux", "amd64")

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
	server, _ := releaseServer(t, version, assetName, archive)
	defer server.Close()

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(server.URL, executable, "linux", "amd64")

	_, err = service.Update(context.Background(), "latest")
	require.Error(t, err)
	content, readErr := os.ReadFile(executable)
	require.NoError(t, readErr)
	require.Equal(t, []byte("old executable"), content)
}

func TestUpdateRejectsUnsupportedPlatformBeforeNetworkAccess(t *testing.T) {
	service := testService("http://127.0.0.1:1", writeExecutable(t, []byte("old executable")), "plan9", "amd64")

	_, err := service.Update(context.Background(), "latest")
	require.ErrorIs(t, err, ErrUnsupportedPlatform)
}

func TestUpdateMissingAssetDoesNotReplaceExecutable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/releases/latest" {
			t.Errorf("unexpected request: %s", request.URL.Path)
		}
		_, _ = fmt.Fprint(writer, `{"tag_name":"v1.2.3","assets":[]}`)
	}))
	defer server.Close()

	executable := writeExecutable(t, []byte("old executable"))
	service := testService(server.URL, executable, "linux", "amd64")

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

func testService(apiBaseURL, executable, goos, goarch string) *Service {
	return &Service{
		apiBaseURL: apiBaseURL,
		httpClient: http.DefaultClient,
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

func releaseServer(t *testing.T, version, assetName string, archive []byte) (*httptest.Server, *[]string) {
	t.Helper()
	digest := sha256.Sum256(archive)
	return releaseServerWithChecksum(t, version, assetName, archive, hex.EncodeToString(digest[:]))
}

func releaseServerWithChecksum(t *testing.T, version, assetName string, archive []byte, checksum string) (*httptest.Server, *[]string) {
	t.Helper()
	requested := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = append(requested, request.URL.Path)
		switch request.URL.Path {
		case "/releases/latest", "/releases/tags/" + version:
			_, _ = fmt.Fprintf(writer, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
				version, assetName, serverURL(request, "/assets/"+assetName), serverURL(request, "/assets/checksums.txt"))
		case "/assets/" + assetName:
			_, _ = writer.Write(archive)
		case "/assets/checksums.txt":
			_, _ = fmt.Fprintf(writer, "%s  %s\n", checksum, assetName)
		default:
			http.NotFound(writer, request)
		}
	}))
	return server, &requested
}

func serverURL(request *http.Request, path string) string {
	return "http://" + request.Host + path
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
