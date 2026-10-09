package npm

import (
	"fmt"
	"path"
	"strings"
)

func tarballStorageKey(packageName, version string) string {
	return path.Join("tarballs", packageName, version+".tgz")
}

func tarballShasumStorageKey(packageName, version string) string {
	return path.Join("tarballs", packageName, version+".sha1")
}

func tarballFilename(packageName, version string) string {
	unscopedName := packageName
	if lastSlash := strings.LastIndex(packageName, "/"); lastSlash >= 0 {
		unscopedName = packageName[lastSlash+1:]
	}
	return fmt.Sprintf("%s-%s.tgz", unscopedName, version)
}

func versionFromTarballFilename(packageName, filename string) (string, error) {
	unscopedName := packageName
	if lastSlash := strings.LastIndex(packageName, "/"); lastSlash >= 0 {
		unscopedName = packageName[lastSlash+1:]
	}
	prefix := unscopedName + "-"
	if !strings.HasPrefix(filename, prefix) {
		return "", fmt.Errorf("filename %q does not match package %q", filename, packageName)
	}
	rest := strings.TrimPrefix(filename, prefix)
	if !strings.HasSuffix(rest, ".tgz") {
		return "", fmt.Errorf("expected .tgz archive")
	}
	version := strings.TrimSuffix(rest, ".tgz")
	if version == "" {
		return "", fmt.Errorf("missing version in filename")
	}
	return version, nil
}
