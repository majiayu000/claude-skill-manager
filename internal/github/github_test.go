package github

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/majiayu000/claude-skill-manager/internal/skill"
)

func TestParseGitHubURLEncodedBranch(t *testing.T) {
	for _, tc := range []struct {
		ref, branch, path, file, name string
		ambiguous                     bool
	}{
		{"feature%2Ffoo", "feature/foo", "", "", "repo", false},
		{"feature%2ffoo/skills/docx", "feature/foo", "skills/docx", "", "docx", false},
		{"feature%2Ffoo/skills/docx/SKILL.md", "feature/foo", "skills/docx", "", "docx", false},
		{"feature%2Ffoo/skills/docx_skill.md", "feature/foo", "", "skills/docx_skill.md", "docx", false},
		{"feature%2Ffoo%2Fbar/skills/my%20skill", "feature/foo/bar", "skills/my skill", "", "my skill", false},
		{"feature%252Ffoo/skills/docx", "feature%2Ffoo", "skills/docx", "", "docx", true},
		{"main/skills/my%20skill", "main", "skills/my skill", "", "my skill", true},
		{"feature/foo/skills/docx", "feature", "foo/skills/docx", "", "docx", true},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			info, err := ParseGitHubURL("https://github.com/owner/repo/tree/" + tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			if info.Owner != "owner" || info.Repo != "repo" || info.Branch != tc.branch || info.Path != tc.path || info.FilePath != tc.file || info.TreeRefAmbiguous != tc.ambiguous || GetSkillName(info) != tc.name {
				t.Fatalf("unexpected parsed ref: %+v, skill name: %q", info, GetSkillName(info))
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadAndExtractHonorsEncodedBranch(t *testing.T) {
	for _, subpath := range []string{"", "/skills/docx", "/missing"} {
		t.Run("root"+subpath, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			selected := "selected branch\n"
			longZip := writeTestZip(t, map[string]string{
				"repo-feature-foo/":                     "",
				"repo-feature-foo/SKILL.md":             selected,
				"repo-feature-foo/skills/docx/SKILL.md": selected,
			})
			shortZip := writeTestZip(t, map[string]string{
				"repo-feature/":                         "",
				"repo-feature/foo/SKILL.md":             "wrong branch\n",
				"repo-feature/foo/skills/docx/SKILL.md": "wrong branch\n",
				"repo-feature/foo/missing/SKILL.md":     "wrong branch\n",
			})
			var requests []string
			restore := downloadClient
			downloadClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Path)
				zipPath := longZip
				if r.URL.Path == "/owner/repo/archive/refs/heads/feature.zip" {
					zipPath = shortZip
				} else if r.URL.Path != "/owner/repo/archive/refs/heads/feature/foo.zip" {
					t.Fatalf("unexpected archive request: %s", r.URL)
				}
				body, err := os.ReadFile(zipPath)
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})}
			t.Cleanup(func() { downloadClient = restore })
			info, err := ParseGitHubURL("https://github.com/owner/repo/tree/feature%2Ffoo" + subpath)
			if err != nil {
				t.Fatal(err)
			}
			finalDir, err := DownloadAndExtract(info, GetSkillName(info), ExtractOptions{})
			if subpath == "/missing" {
				if err == nil || finalDir != "" {
					t.Fatalf("missing selected path must fail, got %q, %v", finalDir, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				body, err := os.ReadFile(filepath.Join(finalDir, "SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
				if string(body) != selected {
					t.Fatalf("installed the wrong branch: %q", body)
				}
			}
			if len(requests) != 1 || requests[0] != "/owner/repo/archive/refs/heads/feature/foo.zip" {
				t.Fatalf("encoded branch must be the only archive requested: %v", requests)
			}
		})
	}
}

func TestDownloadAndExtractEscapesBranchCharacters(t *testing.T) {
	for _, tc := range []struct{ ref, branch string }{
		{"feature%252Ffoo", "feature%2Ffoo"},
		{"feature%23foo", "feature#foo"},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			selected := "selected branch\n"
			zipPath := writeTestZip(t, map[string]string{
				"repo-selected/":         "",
				"repo-selected/SKILL.md": selected,
			})
			body, err := os.ReadFile(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			var requests []string
			restore := downloadClient
			downloadClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Path)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})}
			t.Cleanup(func() { downloadClient = restore })
			info, err := ParseGitHubURL("https://github.com/owner/repo/tree/" + tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			finalDir, err := DownloadAndExtract(info, GetSkillName(info), ExtractOptions{})
			if err != nil {
				t.Fatal(err)
			}
			installed, err := os.ReadFile(filepath.Join(finalDir, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(installed) != selected {
				t.Fatalf("unexpected installed skill: %q", installed)
			}
			wantPath := "/owner/repo/archive/refs/heads/" + tc.branch + ".zip"
			if len(requests) != 1 || requests[0] != wantPath {
				t.Fatalf("archive request changed the branch: %v, want %q", requests, wantPath)
			}
		})
	}
}

func TestParseGitHubURLTrimsDirectorySkillFile(t *testing.T) {
	info, err := ParseGitHubURL("langgenius/dify/.agents/skills/frontend-testing/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != ".agents/skills/frontend-testing" {
		t.Fatalf("unexpected path: %s", info.Path)
	}
	if info.FilePath != "" {
		t.Fatalf("expected directory install, got file path: %s", info.FilePath)
	}
	if name := GetSkillName(info); name != "frontend-testing" {
		t.Fatalf("unexpected skill name: %s", name)
	}
}

func TestParseGitHubURLKeepsSingleSkillFile(t *testing.T) {
	info, err := ParseGitHubURL("redmage123/salesforce/.agents/project_analysis_agent_SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.FilePath != ".agents/project_analysis_agent_SKILL.md" {
		t.Fatalf("unexpected file path: %s", info.FilePath)
	}
	if info.Path != "" {
		t.Fatalf("expected file install, got directory path: %s", info.Path)
	}
	if name := GetSkillName(info); name != "project_analysis_agent" {
		t.Fatalf("unexpected skill name: %s", name)
	}
}

func TestParseGitHubURLDoesNotTreatCommandMarkdownAsSkillFile(t *testing.T) {
	info, err := ParseGitHubURL("udecode/plate/.claude/commands/sync-testing-skill.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.FilePath != "" {
		t.Fatalf("command markdown should not be treated as a skill file: %s", info.FilePath)
	}
	if info.Path != ".claude/commands/sync-testing-skill.md" {
		t.Fatalf("unexpected path: %s", info.Path)
	}
}

func TestDownloadClientHasTimeout(t *testing.T) {
	if downloadClient.Timeout <= 0 {
		t.Fatal("download client must have a bounded timeout")
	}
}

func TestParseGitHubURLBlob(t *testing.T) {
	for _, tc := range []struct {
		input, branch, path, file, name string
	}{
		{"https://github.com/owner/repo/blob/main/skills/docx/SKILL.md", "main", "skills/docx", "", "docx"},
		{"https://github.com/owner/repo/blob/develop/skills/docx/SKILL.md?plain=1#L2", "develop", "skills/docx", "", "docx"},
		{"https://github.com/owner/repo/blob/main/.agents/project_analysis_agent_SKILL.md", "main", "", ".agents/project_analysis_agent_SKILL.md", "project_analysis_agent"},
		{"https://github.com/owner/repo/blob/main/SKILL.md", "main", "", "", "repo"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			info, err := ParseGitHubURL(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if info.Owner != "owner" || info.Repo != "repo" || info.Branch != tc.branch || info.Path != tc.path || info.FilePath != tc.file || GetSkillName(info) != tc.name {
				t.Fatalf("unexpected blob target: %+v, name: %q", info, GetSkillName(info))
			}
		})
	}
}

func TestParseGitHubURLRejectsUnsupportedSections(t *testing.T) {
	for _, section := range []string{"commit/abc", "pull/1", "releases/tag/v1", "issues/1", "unknown/tree/main/skills/docx"} {
		t.Run(section, func(t *testing.T) {
			info, err := ParseGitHubURL("https://github.com/owner/repo/" + section)
			if info != nil || err == nil || err.Error() != "unsupported GitHub URL section: "+strings.Split(section, "/")[0] {
				t.Fatalf("unsupported section must return a named error, got %+v, %v", info, err)
			}
		})
	}
}

func TestParseGitHubURLRejectsIncompleteBlob(t *testing.T) {
	for _, suffix := range []string{"blob", "blob/main", "blob/main/", "blob//skills/docx/SKILL.md"} {
		t.Run(suffix, func(t *testing.T) {
			info, err := ParseGitHubURL("https://github.com/owner/repo/" + suffix)
			if info != nil || err == nil {
				t.Fatalf("incomplete blob URL must fail, got %+v, %v", info, err)
			}
		})
	}
}

func TestParseGitHubURLRootAndTree(t *testing.T) {
	for _, tc := range []struct{ input, path string }{
		{"https://github.com/owner/repo", ""},
		{"https://github.com/owner/repo.git", ""},
		{"https://github.com/owner/repo/?tab=readme#readme", ""},
		{"https://github.com/owner/repo/tree/main/skills/docx/SKILL.md", "skills/docx"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			info, err := ParseGitHubURL(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if info.Owner != "owner" || info.Repo != "repo" || info.Branch != "main" || info.Path != tc.path {
				t.Fatalf("unexpected repository target: %+v", info)
			}
		})
	}
}

func TestInstallBlobURLSelectsNestedSkill(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	zipPath := writeTestZip(t, map[string]string{
		"repo-main/SKILL.md":              "wrong root skill\n",
		"repo-main/skills/docx/SKILL.md":  "selected docx skill\n",
		"repo-main/skills/docx/helper.md": "selected helper\n",
	})
	info, err := ParseGitHubURL("https://github.com/owner/repo/blob/main/skills/docx/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	finalDir, err := installZipAtomically(zipPath, info, GetSkillName(info), ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(finalDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(finalDir) != "docx" || string(body) != "selected docx skill\n" {
		t.Fatalf("blob URL installed the wrong skill: %s, %q", finalDir, body)
	}
	if _, err := os.Stat(filepath.Join(finalDir, "helper.md")); err != nil {
		t.Fatalf("nested skill assets were not installed: %v", err)
	}
}

func TestDownloadToTempFileTimesOut(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() {
		close(block)
		srv.Close()
	}()

	restore := downloadClient
	downloadClient = &http.Client{Timeout: 50 * time.Millisecond}
	defer func() { downloadClient = restore }()

	if _, err := downloadToTempFile(srv.URL); err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}

func TestDownloadToTempFileReportsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := downloadToTempFile(srv.URL)
	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected *httpStatusError, got %v", err)
	}
}

func TestDownloadToTempFileWritesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	defer srv.Close()

	path, err := downloadToTempFile(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(path) }()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "zip-bytes" {
		t.Fatalf("unexpected body: %q", data)
	}
}

type fallbackRoundTripFunc func(*http.Request) (*http.Response, error)

func (f fallbackRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadAndExtractDefaultBranchFallback(t *testing.T) {
	for _, tc := range []struct {
		name          string
		input         string
		mainStatus    int
		masterStatus  int
		transportFail string
		wantFallback  bool
		wantSuccess   bool
	}{
		{name: "main succeeds", input: "owner/repo", mainStatus: 200, wantSuccess: true},
		{name: "forbidden", input: "owner/repo", mainStatus: 403, masterStatus: 200},
		{name: "rate limited", input: "owner/repo", mainStatus: 429, masterStatus: 200},
		{name: "server error", input: "owner/repo", mainStatus: 500, masterStatus: 200},
		{name: "main transport failure", input: "owner/repo", transportFail: "main"},
		{name: "explicit main", input: "https://github.com/owner/repo/tree/main", mainStatus: 404, masterStatus: 200},
		{name: "explicit main path", input: "https://github.com/owner/repo/tree/main/skills/docx/SKILL.md", mainStatus: 404, masterStatus: 200},
		{name: "implicit short repo", input: "owner/repo", mainStatus: 404, masterStatus: 200, wantFallback: true, wantSuccess: true},
		{name: "implicit full repo", input: "https://github.com/owner/repo", mainStatus: 404, masterStatus: 200, wantFallback: true, wantSuccess: true},
		{name: "implicit path", input: "owner/repo/skills/docx/SKILL.md", mainStatus: 404, masterStatus: 200, wantFallback: true, wantSuccess: true},
		{name: "master missing", input: "owner/repo", mainStatus: 404, masterStatus: 404, wantFallback: true},
		{name: "master forbidden", input: "owner/repo", mainStatus: 404, masterStatus: 403, wantFallback: true},
		{name: "master server error", input: "owner/repo", mainStatus: 404, masterStatus: 500, wantFallback: true},
		{name: "master transport failure", input: "owner/repo", mainStatus: 404, transportFail: "master", wantFallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			installed := filepath.Join(home, ".claude", "skills", "docx")
			if err := os.MkdirAll(installed, 0755); err != nil {
				t.Fatal(err)
			}
			original := "---\nname: docx\n---\noriginal install\n"
			if err := os.WriteFile(filepath.Join(installed, "SKILL.md"), []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			archives := make(map[string][]byte)
			for _, branch := range []string{"main", "master"} {
				root := "repo-" + branch + "/"
				body, err := os.ReadFile(writeTestZip(t, map[string]string{
					root:                          "",
					root + "SKILL.md":             branch + " skill\n",
					root + "skills/docx/SKILL.md": branch + " skill\n",
				}))
				if err != nil {
					t.Fatal(err)
				}
				archives[branch] = body
			}
			var requests []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				branch, status := "main", tc.mainStatus
				if r.URL.Path == "/owner/repo/archive/refs/heads/master.zip" {
					branch, status = "master", tc.masterStatus
				} else if tc.name == "explicit main path" && (r.URL.Path == "/owner/repo/archive/refs/heads/main/skills/docx.zip" || r.URL.Path == "/owner/repo/archive/refs/heads/main/skills.zip") {
					http.NotFound(w, r)
					return
				} else if r.URL.Path != "/owner/repo/archive/refs/heads/main.zip" {
					t.Errorf("unexpected archive request: %s", r.URL)
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(status)
				if status == http.StatusOK {
					if _, err := w.Write(archives[branch]); err != nil {
						t.Error(err)
					}
				}
			}))
			defer srv.Close()
			serverURL, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			info, err := ParseGitHubURL(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			restore := downloadClient
			transportErr := errors.New("connection interrupted")
			downloadClient = &http.Client{Transport: fallbackRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Path)
				if info.Branch != "main" {
					t.Errorf("branch changed before download succeeded: %s", info.Branch)
				}
				if tc.transportFail != "" && strings.HasSuffix(r.URL.Path, "/"+tc.transportFail+".zip") {
					return nil, transportErr
				}
				req := r.Clone(r.Context())
				req.URL.Scheme, req.URL.Host = serverURL.Scheme, serverURL.Host
				return srv.Client().Transport.RoundTrip(req)
			})}
			t.Cleanup(func() { downloadClient = restore })
			finalDir, err := DownloadAndExtract(info, "docx", ExtractOptions{AllowReplace: true})
			wantBranch, wantBody := "main", original
			if tc.wantSuccess {
				if err != nil || finalDir != installed {
					t.Errorf("expected successful install at %s, got %q, %v", installed, finalDir, err)
				}
				if tc.wantFallback {
					wantBranch = "master"
				}
				wantBody = wantBranch + " skill\n"
			} else {
				if err == nil || finalDir != "" {
					t.Errorf("expected failed install, got %q, %v", finalDir, err)
				}
				if tc.transportFail == "main" {
					if !errors.Is(err, transportErr) {
						t.Errorf("expected original transport error, got %v", err)
					}
				} else {
					var statusErr *httpStatusError
					wantStatus := http.StatusText(tc.mainStatus)
					if !errors.As(err, &statusErr) || !strings.HasSuffix(statusErr.status, " "+wantStatus) {
						t.Errorf("expected original main status %d %s, got %v", tc.mainStatus, wantStatus, err)
					}
				}
			}
			if info.Branch != wantBranch {
				t.Errorf("branch = %q, want %q", info.Branch, wantBranch)
			}
			gotRequests := strings.Join(requests, ",")
			wantRequests := "/owner/repo/archive/refs/heads/main.zip"
			if tc.wantFallback {
				wantRequests += ",/owner/repo/archive/refs/heads/master.zip"
			} else if tc.name == "explicit main path" {
				// Preserve #51's ambiguous-ref probes without a master fallback.
				wantRequests += ",/owner/repo/archive/refs/heads/main/skills/docx.zip,/owner/repo/archive/refs/heads/main/skills.zip,/owner/repo/archive/refs/heads/main.zip"
			}
			if gotRequests != wantRequests {
				t.Errorf("HTTP requests = %q, want %q", gotRequests, wantRequests)
			}
			body, readErr := os.ReadFile(filepath.Join(installed, "SKILL.md"))
			if readErr != nil || string(body) != wantBody {
				t.Errorf("installed skill = %q, want %q; error %v", body, wantBody, readErr)
			}
		})
	}
}

func TestForceReinstallKeepsExistingSkillOnExtractFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "docx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: docx\ndescription: working install\n---\nkeep me\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(skillDir, "precious.txt")
	if err := os.WriteFile(markerPath, []byte("do-not-delete"), 0644); err != nil {
		t.Fatal(err)
	}

	// Zip has the wrong path / no SKILL.md — extract must fail before swap.
	zipPath := writeTestZip(t, map[string]string{
		"repo-main/other/README.md": "not a skill",
	})

	info := &RepoInfo{
		Owner:  "owner",
		Repo:   "repo",
		Branch: "main",
		Path:   "docx",
	}
	if _, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{}); err == nil {
		t.Fatal("expected extract failure for --force reinstall")
	}

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("original skill directory should still exist: %v", err)
	}
	if string(got) != original {
		t.Fatalf("original SKILL.md changed:\n%s", got)
	}
	marker, err := os.ReadFile(markerPath)
	if err != nil || string(marker) != "do-not-delete" {
		t.Fatalf("original skill contents were disturbed: err=%v marker=%q", err, marker)
	}

	entries, err := os.ReadDir(filepath.Join(home, ".claude", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Fatalf("staging directory leaked: %s", e.Name())
		}
	}
}

func TestAtomicInstallReplacesExistingSkillOnSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "docx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: docx\n---\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "old-only.txt"), []byte("gone"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md":  "---\nname: docx\ndescription: refreshed\n---\nnew\n",
		"repo-main/docx/helper.md": "helper",
	})

	info := &RepoInfo{
		Owner:  "owner",
		Repo:   "repo",
		Branch: "main",
		Path:   "docx",
	}
	if _, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{AllowReplace: true}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "---\nname: docx\ndescription: refreshed\n---\nnew\n" {
		t.Fatalf("unexpected SKILL.md after swap:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "helper.md")); err != nil {
		t.Fatalf("expected helper.md after swap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "old-only.txt")); !os.IsNotExist(err) {
		t.Fatal("expected old-only.txt to be replaced away")
	}
}

func TestRejectParentDirectoryTargetNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md": "---\nname: docx\n---\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docx"}

	for _, name := range []string{".", "..", "a/b", "../x", "x/.."} {
		if _, err := installZipAtomically(zipPath, info, name, ExtractOptions{}); err == nil {
			t.Fatalf("expected invalid skill name %q to be rejected", name)
		}
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("skills root should be untouched after rejected names, got %v", entries)
	}
}

func TestForceReplaceUsesAliasedInstallPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	aliasDir := filepath.Join(home, ".claude", "skills", "alias")
	if err := os.MkdirAll(aliasDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aliasDir, "SKILL.md"), []byte("---\nname: docx\n---\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md": "---\nname: docx\ndescription: refreshed\n---\nnew\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docx"}

	finalDir, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{
		FinalDir:     aliasDir,
		AllowReplace: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if finalDir != aliasDir {
		t.Fatalf("expected install into alias path, got %s", finalDir)
	}

	got, err := os.ReadFile(filepath.Join(aliasDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "refreshed") {
		t.Fatalf("alias install was not replaced: %s", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "docx")); !os.IsNotExist(err) {
		t.Fatal("expected no duplicate skills/docx directory")
	}
}

func TestForceInstallByDirectoryNameDoesNotReplaceAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	aliasDir := filepath.Join(home, ".claude", "skills", "pdf-tools")
	if err := os.MkdirAll(aliasDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: pdf\ndescription: PDF tools\n---\nold\n"
	if err := os.WriteFile(filepath.Join(aliasDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/pdf/SKILL.md": "---\nname: pdf\ndescription: other pdf skill\n---\nnew\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "pdf"}

	// sk install <src> --name pdf --force: occupy skills/pdf only.
	finalDir, err := installZipAtomically(zipPath, info, "pdf", ExtractOptions{AllowReplace: true})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".claude", "skills", "pdf")
	if finalDir != want {
		t.Fatalf("expected install into %s, got %s", want, finalDir)
	}

	got, err := os.ReadFile(filepath.Join(aliasDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("pdf-tools must survive force install targeting pdf: %v", err)
	}
	if string(got) != original {
		t.Fatalf("pdf-tools was modified:\n%s", got)
	}
	installed, err := os.ReadFile(filepath.Join(want, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "other pdf skill") {
		t.Fatalf("expected new skill at skills/pdf, got %s", installed)
	}
}

func TestRefuseUnforcedNonSkillCollision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(skillsDir, "docx")
	if err := os.WriteFile(collision, []byte("not a skill dir"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := writeTestZip(t, map[string]string{
		"repo-main/docx/SKILL.md": "---\nname: docx\n---\n",
	})
	info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docx"}

	if _, err := installZipAtomically(zipPath, info, "docx", ExtractOptions{}); err == nil {
		t.Fatal("expected collision with non-skill path to be refused")
	}

	got, err := os.ReadFile(collision)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "not a skill dir" {
		t.Fatalf("collision path was modified: %q", got)
	}
}

func TestReplaceRestoresBackupWhenRenameFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "docx")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: docx\n---\nkeep\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// stagingDir points at a missing path so Rename(staging -> final) fails
	// after the existing install has already been moved aside.
	stagingMissing := filepath.Join(home, ".claude", "skills", ".docx.staging-missing")
	err := replaceSkillDir(skillDir, stagingMissing, true)
	if err == nil {
		t.Fatal("expected rename failure")
	}

	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("original skill should be restored after failed rename: %v", err)
	}
	if string(got) != original {
		t.Fatalf("restored contents mismatch: %q", got)
	}
}

func TestValidateSkillName(t *testing.T) {
	if err := validateSkillName("docx"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "victim.", "victim ", ". "} {
		if err := validateSkillName(name); err == nil {
			t.Fatalf("expected %q to be invalid", name)
		}
	}
}

func writeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "skill.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDownloadAndExtractHonorsEncodedBlobBranch(t *testing.T) {
	for _, subpath := range []string{"", "/skills/docx", "/missing"} {
		t.Run("root"+subpath, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			selected := "selected branch\n"
			longZip := writeTestZip(t, map[string]string{
				"repo-feature-foo/":                     "",
				"repo-feature-foo/SKILL.md":             selected,
				"repo-feature-foo/skills/docx/SKILL.md": selected,
			})
			shortZip := writeTestZip(t, map[string]string{
				"repo-feature/":                         "",
				"repo-feature/foo/SKILL.md":             "wrong branch\n",
				"repo-feature/foo/skills/docx/SKILL.md": "wrong branch\n",
				"repo-feature/foo/missing/SKILL.md":     "wrong branch\n",
			})
			var requests []string
			restore := downloadClient
			downloadClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Path)
				zipPath := longZip
				if r.URL.Path == "/owner/repo/archive/refs/heads/feature.zip" {
					zipPath = shortZip
				} else if r.URL.Path != "/owner/repo/archive/refs/heads/feature/foo.zip" {
					t.Fatalf("unexpected archive request: %s", r.URL)
				}
				body, err := os.ReadFile(zipPath)
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})}
			t.Cleanup(func() { downloadClient = restore })
			info, err := ParseGitHubURL("https://github.com/owner/repo/blob/feature%2Ffoo" + subpath + "/SKILL.md")
			if err != nil {
				t.Fatal(err)
			}
			finalDir, err := DownloadAndExtract(info, GetSkillName(info), ExtractOptions{})
			if subpath == "/missing" {
				if err == nil || finalDir != "" {
					t.Fatalf("missing selected path must fail, got %q, %v", finalDir, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				body, err := os.ReadFile(filepath.Join(finalDir, "SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
				if string(body) != selected {
					t.Fatalf("installed the wrong branch: %q", body)
				}
			}
			if len(requests) != 1 || requests[0] != "/owner/repo/archive/refs/heads/feature/foo.zip" {
				t.Fatalf("encoded branch must be the only archive requested: %v", requests)
			}
		})
	}
}

func TestInstallPreservesDottedInstallerLikeSkillNames(t *testing.T) {
	for _, name := range []string{".docs.staging-keep", ".docs.backup-keep"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			body := "---\nname: docs\n---\nkeep\n"
			zipPath := writeTestZip(t, map[string]string{"repo-main/docs/SKILL.md": body})
			info := &RepoInfo{Owner: "owner", Repo: "repo", Branch: "main", Path: "docs"}
			for _, replace := range []bool{false, true} {
				dir, err := installZipAtomically(zipPath, info, name, ExtractOptions{AllowReplace: replace})
				if err != nil {
					t.Fatal(err)
				}
				// A later installation also invokes orphan recovery.
				if _, err := installZipAtomically(zipPath, info, "other", ExtractOptions{AllowReplace: replace}); err != nil {
					t.Fatal(err)
				}
				skills, err := skill.List()
				if err != nil {
					t.Fatal(err)
				}
				if len(skills) != 2 || !skill.Exists(name) {
					t.Errorf("expected dotted install and sibling to remain, got %#v", skills)
				}
				got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
				if err != nil || string(got) != body {
					t.Fatalf("installed contents must remain at original path: got %q, error %v", got, err)
				}
			}
		})
	}
}

func TestUnencodedBlobRefRecovery(t *testing.T) {
	for _, tc := range []struct {
		ref, branch, file string
		status            int
	}{
		{"feature/foo/skills/docx/SKILL.md", "feature/foo/skills", "docx/SKILL.md", 200},
		{"feature/foo/skills/docx_skill.md", "feature/foo", "skills/docx_skill.md", 200},
		{"feature/foo/skills/docx/SKILL.md", "feature/foo", "skills/docx/SKILL.md", 404},
		{"feature/foo/skills/docx/SKILL.md", "feature/foo", "skills/docx/SKILL.md", 502},
	} {
		t.Run(tc.ref+http.StatusText(tc.status), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			zipPath := writeTestZip(t, map[string]string{"repo-selected/": "", "repo-selected/" + tc.file: "selected blob\n"})
			data, err := os.ReadFile(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			var requests []string
			restore := downloadClient
			downloadClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.Path)
				code := tc.status
				if tc.status == 200 && r.URL.Path != "/owner/repo/archive/refs/heads/"+tc.branch+".zip" {
					code = 404
				}
				return &http.Response{StatusCode: code, Status: strings.TrimSpace(strings.Join([]string{strconv.Itoa(code), http.StatusText(code)}, " ")), Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})}
			t.Cleanup(func() { downloadClient = restore })
			info, err := ParseGitHubURL("https://github.com/owner/repo/blob/" + tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			dir, err := DownloadAndExtract(info, GetSkillName(info), ExtractOptions{})
			if tc.status == 200 {
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
				if err != nil || string(got) != "selected blob\n" {
					t.Fatalf("wrong blob: %q %v", got, err)
				}
				for _, p := range requests {
					if strings.Contains(p, "SKILL.md.zip") || strings.Contains(p, "docx_skill.md.zip") {
						t.Fatalf("blob file became branch: %v", requests)
					}
				}
			} else {
				var statusErr *httpStatusError
				if dir != "" || !errors.As(err, &statusErr) || statusErr.status != strconv.Itoa(tc.status)+" "+http.StatusText(tc.status) {
					t.Fatalf("original status error lost: %q %v", dir, err)
				}
				if tc.status == 502 && len(requests) != 1 {
					t.Fatalf("non-404 must not probe: %v", requests)
				}
			}
		})
	}
}

func TestBlobRejectsNonSkillFile(t *testing.T) {
	for _, p := range []string{"README.md", "helper.py", "skills/docx/README.md"} {
		if info, err := ParseGitHubURL("https://github.com/owner/repo/blob/main/" + p); info != nil || err == nil || !strings.Contains(err.Error(), "unsupported GitHub blob file") {
			t.Fatalf("non-skill blob accepted: %+v %v", info, err)
		}
	}
}
