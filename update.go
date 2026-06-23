package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version courante de l'app. À incrémenter avant chaque tag de release.
var Version = "1.0.1"

const repoSlug = "Gandalfleblanc/LiHDL-Search-Films-FR"

type UpdateInfo struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	URL       string `json:"url"`   // page de la release
	Notes     string `json:"notes"` // corps de la release
	Error     string `json:"error"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// GetVersion renvoie la version courante (affichée dans l'UI).
func (a *App) GetVersion() string { return Version }

func fetchLatest() (*ghRelease, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repoSlug+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LiHDL-Search-Films-FR")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// CheckUpdate interroge la dernière release GitHub et compare à la version courante.
func (a *App) CheckUpdate() UpdateInfo {
	info := UpdateInfo{Current: Version}
	rel, err := fetchLatest()
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.URL = rel.HTMLURL
	info.Notes = rel.Body
	info.Available = versionLess(Version, rel.TagName)
	return info
}

// DoUpdate télécharge et installe la dernière version (macOS), puis relance l'app.
// Sur les autres OS, ouvre la page de téléchargement.
func (a *App) DoUpdate() UpdateInfo {
	info := a.CheckUpdate()
	if info.Error != "" {
		return info
	}
	if !info.Available {
		info.Error = "Déjà à jour."
		return info
	}
	rel, err := fetchLatest()
	if err != nil {
		info.Error = err.Error()
		return info
	}

	if runtime.GOOS != "darwin" {
		wruntime.BrowserOpenURL(a.ctx, rel.HTMLURL)
		info.Error = "Mise à jour auto disponible sur macOS uniquement — page de téléchargement ouverte dans le navigateur."
		return info
	}

	// Choix de l'asset selon l'architecture.
	suffix := "macos-arm64.zip"
	if runtime.GOARCH == "amd64" {
		suffix = "macos-amd64.zip"
	}
	var assetURL string
	for _, as := range rel.Assets {
		if strings.HasSuffix(as.Name, suffix) {
			assetURL = as.URL
			break
		}
	}
	if assetURL == "" {
		info.Error = "Aucun binaire macOS (" + suffix + ") dans la release."
		return info
	}

	a.progress("Téléchargement de la mise à jour…")
	zipPath, err := downloadTemp(assetURL)
	if err != nil {
		info.Error = "Téléchargement : " + err.Error()
		return info
	}

	tmpDir, err := os.MkdirTemp("", "lihdl-update")
	if err != nil {
		info.Error = err.Error()
		return info
	}
	a.progress("Décompression…")
	if out, err := exec.Command("ditto", "-x", "-k", zipPath, tmpDir).CombinedOutput(); err != nil {
		info.Error = "Décompression : " + strings.TrimSpace(string(out))
		return info
	}
	newApp := findApp(tmpDir)
	if newApp == "" {
		info.Error = "Bundle .app introuvable dans l'archive."
		return info
	}

	exe, err := os.Executable()
	if err != nil {
		info.Error = err.Error()
		return info
	}
	// exe = .../X.app/Contents/MacOS/binaire  -> remonter de 3 niveaux
	oldApp := filepath.Dir(filepath.Dir(filepath.Dir(exe)))

	// Script détaché : attend la fermeture de l'app, remplace le bundle, relance.
	script := fmt.Sprintf(`#!/bin/bash
sleep 1
while kill -0 %d 2>/dev/null; do sleep 0.3; done
rm -rf "%s"
ditto "%s" "%s"
xattr -cr "%s"
open "%s"
`, os.Getpid(), oldApp, newApp, oldApp, oldApp, oldApp)

	sp := filepath.Join(tmpDir, "swap.sh")
	if err := os.WriteFile(sp, []byte(script), 0o755); err != nil {
		info.Error = err.Error()
		return info
	}
	if err := exec.Command("/bin/bash", sp).Start(); err != nil {
		info.Error = err.Error()
		return info
	}

	a.progress("Installation… l'app va redémarrer.")
	go func() {
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}()
	return info
}

// ---------- helpers update ----------

func downloadTemp(url string) (string, error) {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp("", "lihdl-*.zip")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func findApp(dir string) string {
	var found string
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if fi.IsDir() && strings.HasSuffix(p, ".app") && !strings.Contains(p, "__MACOSX") {
			found = p
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func versionLess(a, b string) bool {
	av, bv := parseVer(a), parseVer(b)
	for i := 0; i < len(av) || i < len(bv); i++ {
		x, y := 0, 0
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func parseVer(s string) []int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		nums = append(nums, n)
	}
	return nums
}
