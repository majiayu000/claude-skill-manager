package github

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/config"
)

// downloadClient is used for repository archive downloads. Archives can be
// large, so it allows more time than the registry client, but it must stay
// bounded so a stalled connection cannot hang the CLI forever.
var downloadClient = &http.Client{Timeout: 120 * time.Second}

// httpStatusError reports a non-200 response, so callers can distinguish a
// missing branch (worth retrying as "master") from a transport failure.
type httpStatusError struct {
	status string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("download failed with status: %s", e.status)
}

func archiveURL(info *RepoInfo) string {
	return fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip",
		info.Owner, info.Repo, info.Branch)
}

// downloadToTempFile fetches url into a temp file and returns its path.
// The caller owns the file and must remove it.
func downloadToTempFile(url string) (string, error) {
	resp, err := downloadClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", &httpStatusError{status: resp.Status}
	}

	tmpFile, err := os.CreateTemp("", "sk-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to save zip: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to save zip: %w", err)
	}

	return tmpFile.Name(), nil
}

// RepoInfo contains parsed GitHub repository information
type RepoInfo struct {
	Owner            string
	Repo             string
	Path             string // subdirectory path (for monorepo skills)
	FilePath         string // direct SKILL.md-like file path
	Branch           string
	TreeRef          string
	TreeRefAmbiguous bool
	FullURL          string
	CloneURL         string
}

// ParseGitHubURL parses various GitHub URL formats
// Supports:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo/tree/branch/path
//   - owner/repo
//   - owner/repo/path
func ParseGitHubURL(input string) (*RepoInfo, error) {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, "/")
	input = strings.TrimSuffix(input, ".git")

	info := &RepoInfo{}

	// Full GitHub URL
	if strings.HasPrefix(input, "https://github.com/") {
		raw := input
		if idx := strings.IndexAny(raw, "?#"); idx != -1 {
			raw = raw[:idx]
		}

		if strings.Contains(raw, "/tree/") {
			u, err := url.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid GitHub URL format: %s", input)
			}

			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) < 4 || parts[2] != "tree" {
				return nil, fmt.Errorf("invalid GitHub URL format: %s", input)
			}

			info.Owner = parts[0]
			info.Repo = parts[1]

			treeParts := parts[3:]
			if len(treeParts) == 0 {
				return nil, fmt.Errorf("invalid GitHub URL format: %s", input)
			}

			info.TreeRef = strings.Join(treeParts, "/")
			if strings.Contains(info.TreeRef, "%2F") || strings.Contains(info.TreeRef, "%2f") {
				decoded, err := url.PathUnescape(info.TreeRef)
				if err != nil {
					return nil, fmt.Errorf("invalid GitHub URL format: %s", input)
				}
				info.Branch = decoded
				info.Path = ""
			} else {
				info.Branch = treeParts[0]
				if len(treeParts) > 1 {
					info.Path = strings.Join(treeParts[1:], "/")
					info.TreeRefAmbiguous = true
				}
			}
		} else {
			// Pattern: https://github.com/owner/repo
			simplePattern := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)`)
			if matches := simplePattern.FindStringSubmatch(input); len(matches) >= 3 {
				info.Owner = matches[1]
				info.Repo = matches[2]
				info.Branch = "main" // default
			} else {
				return nil, fmt.Errorf("invalid GitHub URL format: %s", input)
			}
		}
	} else {
		// Short format: owner/repo or owner/repo/path
		parts := strings.Split(input, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid format, expected owner/repo: %s", input)
		}
		info.Owner = parts[0]
		info.Repo = parts[1]
		info.Branch = "main"
		if len(parts) > 2 {
			info.Path = strings.Join(parts[2:], "/")
		}
	}

	normalizeSkillPath(info)

	info.FullURL = fmt.Sprintf("https://github.com/%s/%s", info.Owner, info.Repo)
	info.CloneURL = info.FullURL + ".git"

	return info, nil
}

func normalizeSkillPath(info *RepoInfo) {
	info.Path = strings.Trim(strings.ReplaceAll(info.Path, "\\", "/"), "/")
	if info.Path == "" {
		return
	}

	base := pathBase(info.Path)
	lowerBase := strings.ToLower(base)
	if base == "SKILL.md" {
		info.Path = pathDir(info.Path)
		return
	}
	if lowerBase == "skill.md" || strings.HasSuffix(lowerBase, "_skill.md") {
		info.FilePath = info.Path
		info.Path = ""
	}
}

// ExtractOptions controls where a downloaded skill is installed and whether an
// existing path may be replaced. FinalDir, when set, must already be an
// identified skill directory being force-reinstalled (for example an aliased
// install whose directory basename differs from the front-matter name).
type ExtractOptions struct {
	FinalDir     string
	AllowReplace bool
}

// DownloadAndExtract downloads a repository and extracts to the skills directory.
// Extraction always goes into a staging directory under the skills root; the
// existing skill (if any) is replaced only after the new tree validates.
// On any failure before that swap, the previous install is left intact.
// It returns the final install directory on success.
func DownloadAndExtract(info *RepoInfo, targetName string, opts ExtractOptions) (string, error) {
	if err := config.EnsureSkillsDir(); err != nil {
		return "", fmt.Errorf("failed to create skills directory: %w", err)
	}

	// Download as zip
	zipPath, err := downloadToTempFile(archiveURL(info))

	// Try 'master' branch if 'main' is missing
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) && info.Branch == "main" {
		info.Branch = "master"
		zipPath, err = downloadToTempFile(archiveURL(info))
	}
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(zipPath) }()

	finalDir, err := installZipAtomically(zipPath, info, targetName, opts)
	if err != nil && info.TreeRef != "" && info.TreeRefAmbiguous {
		if resolved, resolveErr := tryResolveAmbiguousTreeRef(info, targetName, opts); resolveErr == nil {
			return resolved, nil
		}
		return "", fmt.Errorf("%w (if your branch contains '/', URL-encode it, e.g. feature%%2Ffoo)", err)
	}
	return finalDir, err
}

// installZipAtomically extracts a downloaded zip into a staging directory under
// the skills root, validates it, then swaps it into the final install path.
// On any failure before the swap, an existing skill at that path is left untouched.
func installZipAtomically(zipPath string, info *RepoInfo, targetName string, opts ExtractOptions) (string, error) {
	if err := config.EnsureSkillsDir(); err != nil {
		return "", fmt.Errorf("failed to create skills directory: %w", err)
	}

	skillsDir := config.GetSkillsDir()
	finalDir, err := resolveFinalSkillDir(skillsDir, targetName, opts.FinalDir)
	if err != nil {
		return "", err
	}

	stagingDir, err := os.MkdirTemp(skillsDir, "."+filepath.Base(finalDir)+".staging-")
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}
	swapped := false
	defer func() {
		if !swapped {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	if err := extractZipWithFallbacks(zipPath, stagingDir, info); err != nil {
		return "", fmt.Errorf("failed to extract: %w", err)
	}
	if err := replaceSkillDir(finalDir, stagingDir, opts.AllowReplace); err != nil {
		return "", err
	}
	swapped = true
	return finalDir, nil
}

// resolveFinalSkillDir validates targetName (or an explicit FinalDir override)
// so the install path is always a single safe component strictly under skillsDir.
func resolveFinalSkillDir(skillsDir, targetName, overrideFinalDir string) (string, error) {
	skillsDir = filepath.Clean(skillsDir)

	var finalDir string
	if overrideFinalDir != "" {
		finalDir = filepath.Clean(overrideFinalDir)
	} else {
		if err := validateSkillName(targetName); err != nil {
			return "", err
		}
		finalDir = filepath.Join(skillsDir, targetName)
	}

	if !isStrictlyWithinDir(skillsDir, finalDir) {
		return "", fmt.Errorf("refusing to install outside skills directory: %s", finalDir)
	}
	if err := validateSkillName(filepath.Base(finalDir)); err != nil {
		return "", err
	}
	return finalDir, nil
}

func validateSkillName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid skill name %q", name)
	}
	if strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return fmt.Errorf("invalid skill name %q: must be a single path component", name)
	}
	if filepath.Base(name) != name || filepath.Clean(name) != name {
		return fmt.Errorf("invalid skill name %q: must be a single path component", name)
	}
	return nil
}

// isStrictlyWithinDir reports whether target is a path strictly contained under
// root (root itself is rejected).
func isStrictlyWithinDir(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// extractZipWithFallbacks extracts into stagingDir, trying common alternate
// skill paths when the primary path does not contain a valid skill.
func extractZipWithFallbacks(zipPath, stagingDir string, info *RepoInfo) error {
	err := extractZip(zipPath, stagingDir, info)
	if err == nil || info.Path == "" {
		return err
	}

	// If path doesn't work, try common skill locations
	// e.g., "docx" -> "skills/docx" for anthropics/skills repo
	alternativePaths := []string{
		"skills/" + info.Path,
		"skill/" + info.Path,
	}

	for _, altPath := range alternativePaths {
		infoCopy := *info
		infoCopy.Path = altPath
		_ = os.RemoveAll(stagingDir)
		if err = extractZip(zipPath, stagingDir, &infoCopy); err == nil {
			return nil
		}
	}
	return err
}

// replaceSkillDir swaps a validated staging directory into the final skill
// path. An existing install is moved aside first and only deleted after the
// staged rename succeeds, so a failed rename restores the previous tree.
// Unforced collisions with any pre-existing path are rejected.
func replaceSkillDir(finalDir, stagingDir string, allowReplace bool) error {
	_, err := os.Lstat(finalDir)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect install path: %w", err)
	}

	if exists && !allowReplace {
		return fmt.Errorf("refusing to overwrite existing path %s (not a force-reinstall of an installed skill)", finalDir)
	}

	var backupDir string
	if exists {
		backupDir, err = os.MkdirTemp(filepath.Dir(finalDir), "."+filepath.Base(finalDir)+".backup-")
		if err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
		// MkdirTemp created an empty dir; remove it so Rename can claim the name.
		if err := os.Remove(backupDir); err != nil {
			return fmt.Errorf("failed to prepare backup directory: %w", err)
		}
		if err := os.Rename(finalDir, backupDir); err != nil {
			return fmt.Errorf("failed to move existing skill aside: %w", err)
		}
	}

	if err := os.Rename(stagingDir, finalDir); err != nil {
		if backupDir != "" {
			if restoreErr := os.Rename(backupDir, finalDir); restoreErr != nil {
				return fmt.Errorf("failed to install staged skill: %v (also failed to restore previous install: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("failed to install staged skill: %w", err)
	}

	if backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}

func tryResolveAmbiguousTreeRef(info *RepoInfo, targetName string, opts ExtractOptions) (string, error) {
	parts := strings.Split(info.TreeRef, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("ambiguous tree ref has insufficient parts")
	}

	// Try longer branch candidates first.
	for i := len(parts); i >= 1; i-- {
		branch := strings.Join(parts[:i], "/")
		path := ""
		if i < len(parts) {
			path = strings.Join(parts[i:], "/")
		}

		infoCopy := *info
		infoCopy.Branch = branch
		infoCopy.Path = path

		zipPath, err := downloadToTempFile(archiveURL(&infoCopy))
		if err != nil {
			continue
		}
		finalDir, err := installZipAtomically(zipPath, &infoCopy, targetName, opts)
		_ = os.Remove(zipPath)
		if err == nil {
			return finalDir, nil
		}
	}

	return "", fmt.Errorf("unable to resolve tree ref")
}

// extractZip extracts the zip file to target directory
func extractZip(zipPath, targetDir string, info *RepoInfo) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Find the actual root prefix from the zip (it might vary)
	var rootPrefix string
	for _, f := range r.File {
		// First entry should be the root directory
		if strings.Count(f.Name, "/") == 1 && strings.HasSuffix(f.Name, "/") {
			rootPrefix = f.Name
			break
		}
	}

	if rootPrefix == "" {
		// Fallback to expected format
		rootPrefix = fmt.Sprintf("%s-%s/", info.Repo, info.Branch)
	}

	if info.FilePath != "" {
		return extractSkillFile(r, rootPrefix, targetDir, info.FilePath)
	}

	subPath := ""
	if info.Path != "" {
		subPath = info.Path + "/"
	}

	fullPrefix := rootPrefix + subPath

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	extractedFiles := 0
	for _, f := range r.File {
		// Skip files not in the target path
		if !strings.HasPrefix(f.Name, fullPrefix) {
			continue
		}

		// Calculate relative path
		relPath := strings.TrimPrefix(f.Name, fullPrefix)
		if relPath == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, relPath)
		if !isWithinDir(targetDir, targetPath) {
			return fmt.Errorf("zip entry escapes target dir: %s", relPath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, f.Mode())
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Extract file
		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
		extractedFiles++
	}

	// Verify SKILL.md exists
	skillMdPath := filepath.Join(targetDir, "SKILL.md")
	if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
		// Clean up
		os.RemoveAll(targetDir)
		if extractedFiles == 0 {
			return fmt.Errorf("no files found at path '%s' - check if the path is correct", info.Path)
		}
		return fmt.Errorf("no SKILL.md found - this doesn't appear to be a valid skill")
	}

	return nil
}

func extractSkillFile(r *zip.ReadCloser, rootPrefix, targetDir, filePath string) error {
	wanted := rootPrefix + strings.Trim(filePath, "/")

	for _, f := range r.File {
		if f.Name != wanted || f.FileInfo().IsDir() {
			continue
		}

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, "SKILL.md")
		if !isWithinDir(targetDir, targetPath) {
			return fmt.Errorf("zip entry escapes target dir: %s", filePath)
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			if closeErr := outFile.Close(); closeErr != nil {
				return closeErr
			}
			return err
		}
		_, err = io.Copy(outFile, rc)
		if closeErr := rc.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := outFile.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		return err
	}

	_ = os.RemoveAll(targetDir)
	return fmt.Errorf("no file found at path '%s' - check if the path is correct", filePath)
}

func isWithinDir(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// GetSkillName determines the skill name from RepoInfo
func GetSkillName(info *RepoInfo) string {
	if info.FilePath != "" {
		base := pathBase(info.FilePath)
		lowerBase := strings.ToLower(base)
		if lowerBase == "skill.md" {
			dir := pathDir(info.FilePath)
			if dir != "" {
				return pathBase(dir)
			}
		}
		name := strings.TrimSuffix(base, filepath.Ext(base))
		name = strings.TrimSuffix(strings.TrimSuffix(name, "_SKILL"), "-SKILL")
		name = strings.TrimSuffix(strings.TrimSuffix(name, "_skill"), "-skill")
		return name
	}
	if info.Path != "" {
		// Use the last part of the path
		parts := strings.Split(info.Path, "/")
		return parts[len(parts)-1]
	}
	return info.Repo
}

func pathBase(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return parts[len(parts)-1]
}

func pathDir(path string) string {
	cleaned := strings.Trim(path, "/")
	idx := strings.LastIndex(cleaned, "/")
	if idx == -1 {
		return ""
	}
	return cleaned[:idx]
}
