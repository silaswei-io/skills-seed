package analyzer

import (
	"path/filepath"
	"sort"
	"strings"
)

func normalizeFileSelectionPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = cleanFileSelectionPath(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func cleanFileSelectionPath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "./")
	path = strings.Trim(path, "/")
	if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func fileSelectionPathSignals(path string) []string {
	if isLowValueSelectionPath(path) {
		return nil
	}
	seen := make(map[string]bool)
	add := func(signal string) {
		if signal != "" {
			seen[signal] = true
		}
	}

	lowerPath := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	switch base {
	case "main.go", "main.ts", "main.tsx", "main.js", "main.jsx",
		"app.go", "app.ts", "app.tsx", "app.js", "app.jsx",
		"server.go", "server.ts", "server.js":
		add("entry_file_name")
	case "route.ts", "route.tsx", "route.js", "route.jsx",
		"page.tsx", "page.jsx", "layout.tsx", "layout.jsx",
		"middleware.ts", "middleware.js":
		add("web_entry_file_name")
	case "package.json", "go.mod", "cargo.toml", "pom.xml", "pyproject.toml":
		add("project_manifest")
	}
	if isModuleRegistrationIndex(lowerPath, base) {
		add("module_registration")
	}

	switch {
	case strings.HasPrefix(lowerPath, "cmd/") || strings.Contains(lowerPath, "/cmd/"):
		add("command_entry_path")
	case strings.Contains(lowerPath, "/api/") || strings.HasPrefix(lowerPath, "api/"):
		add("api_entry_path")
	case strings.Contains(lowerPath, "/routes/") || strings.Contains(lowerPath, "/route/"):
		add("route_entry_path")
	case strings.Contains(lowerPath, "/controllers/") || strings.Contains(lowerPath, "/controller/"):
		add("controller_entry_path")
	case strings.Contains(lowerPath, "/handlers/") || strings.Contains(lowerPath, "/handler/"):
		add("handler_entry_path")
	case strings.Contains(lowerPath, "/subscribers/") || strings.Contains(lowerPath, "/subscriber/"):
		add("event_entry_path")
	case strings.Contains(lowerPath, "/workflows/") || strings.Contains(lowerPath, "/workflow/"):
		add("workflow_entry_path")
	case strings.Contains(lowerPath, "/jobs/") || strings.Contains(lowerPath, "/workers/") || strings.Contains(lowerPath, "/tasks/"):
		add("background_entry_path")
	case isBackendRegistrationPath(lowerPath):
		add("lifecycle_registration_path")
	}

	switch {
	case strings.Contains(stem, "route") || strings.Contains(stem, "router"):
		add("route_file_name")
	case strings.Contains(stem, "handler") || strings.Contains(stem, "controller"):
		add("request_handler_file_name")
	case strings.Contains(stem, "worker") || strings.Contains(stem, "job") || strings.Contains(stem, "scheduler"):
		add("background_file_name")
	case strings.Contains(stem, "workflow"):
		add("business_flow_file_name")
	case isBackendServicePath(lowerPath, base):
		add("business_service_file")
	case strings.Contains(stem, "bootstrap") || isBackendRegistrationPath(lowerPath) && (strings.Contains(stem, "provider") || strings.Contains(stem, "plugin")):
		add("lifecycle_file_name")
	}

	return sortedFileSelectionStrings(seen)
}

func isLowValueSelectionPath(path string) bool {
	return isGeneratedSelectionPath(path) || isTestSelectionPath(path)
}

func isGeneratedSelectionPath(path string) bool {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(lower, "/generated/") ||
		strings.Contains(lower, "/oas-output/") ||
		strings.Contains(lower, "/typedoc-json-output/") ||
		strings.Contains(lower, "/dml-output/") ||
		strings.Contains(lower, "/specs/") ||
		strings.Contains(lower, "/schemas/") ||
		strings.Contains(lower, "/icons/") ||
		strings.Contains(lower, "/translations/") {
		return true
	}
	return strings.HasSuffix(base, ".snap") || strings.HasSuffix(base, ".snapshot")
}

func isTestSelectionPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "/__tests__/") ||
		strings.Contains(lower, "/fixtures/") ||
		strings.Contains(lower, "/__fixtures__/") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, ".test.")
}

func isModuleRegistrationIndex(lowerPath, base string) bool {
	if base != "index.ts" && base != "index.js" && base != "index.go" {
		return false
	}
	return strings.Contains(lowerPath, "/src/modules/") || strings.Contains(lowerPath, "/packages/modules/")
}

func isBackendServicePath(lowerPath, base string) bool {
	if base != "service.ts" && base != "service.js" && base != "module-service.ts" && !strings.HasSuffix(base, "-service.ts") && !strings.HasSuffix(base, "-service.js") {
		return false
	}
	return strings.Contains(lowerPath, "/src/modules/") ||
		strings.Contains(lowerPath, "/packages/modules/") ||
		strings.Contains(lowerPath, "/packages/medusa/src/") ||
		strings.Contains(lowerPath, "/packages/core/")
}

func isBackendRegistrationPath(lowerPath string) bool {
	if !(strings.Contains(lowerPath, "/loaders/") || strings.Contains(lowerPath, "/plugins/") || strings.Contains(lowerPath, "/providers/")) {
		return false
	}
	return strings.Contains(lowerPath, "/packages/medusa/") ||
		strings.Contains(lowerPath, "/packages/modules/") ||
		strings.Contains(lowerPath, "/integration-tests/http/src/")
}

func fileSelectionModule(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "."
	}
	if len(parts) == 1 {
		return "."
	}
	if isFileSelectionWorkspaceDir(parts[0]) && len(parts) >= 2 {
		return parts[0] + "/" + parts[1] + "/"
	}
	return parts[0] + "/"
}

func isFileSelectionWorkspaceDir(name string) bool {
	switch name {
	case "apps", "cmd", "examples", "integration-tests", "internal", "packages", "plugins", "services", "www":
		return true
	default:
		return false
	}
}

func fileSelectionKind(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".yaml", ".yml", ".toml", ".xml":
		return "config-or-data"
	case ".sql", ".graphql", ".gql", ".proto", ".api":
		return "schema-or-contract"
	default:
		if ext == "" {
			return "unknown"
		}
		return "source"
	}
}

func sortedFileSelectionStrings(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
