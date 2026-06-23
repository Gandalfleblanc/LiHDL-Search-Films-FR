package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const tmdbBase = "https://api.themoviedb.org/3"

// Libellé affiché -> sous-chaînes recherchées dans le nom du provider TMDB.
var providerMatch = map[string][]string{
	"Netflix":     {"netflix"},
	"Prime Video": {"amazon prime video", "prime video"},
	"Orange":      {"orange"},
	"Canal+":      {"canal+", "mycanal", "canal vod"},
}

// PlatformList expose l'ordre d'affichage des plateformes au frontend.
var PlatformList = []string{"Netflix", "Prime Video", "Orange", "Canal+"}

// App struct
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ---------- Types exposés au frontend ----------

type Film struct {
	TMDBID     int      `json:"tmdb_id"`
	Title      string   `json:"title"`
	Year       string   `json:"year"`
	Platforms  []string `json:"platforms"`
	Resolution string   `json:"resolution"` // "4K" / "HD" / "SD" / "" (via JustWatch)
	VF         string   `json:"vf"`         // "oui" / "non" / "inconnu" / "" (via JustWatch)
}

type GenerateResult struct {
	Films []Film `json:"films"`
	Count int    `json:"count"`
	Error string `json:"error"`
}

type Settings struct {
	Token     string   `json:"token"`
	UseBearer bool     `json:"useBearer"`
	Platforms []string `json:"platforms"`
	Monetize  []string `json:"monetize"`
	Criteria  string   `json:"criteria"` // "origin" | "language" | "all"
	Enrich    bool     `json:"enrich"`   // enrichissement JustWatch (résolution + VF)
}

// ---------- Client TMDB ----------

type tmdbClient struct {
	bearer, apiKey string
	http           *http.Client
}

func (c *tmdbClient) get(path string, q url.Values, out any) error {
	if c.apiKey != "" {
		q.Set("api_key", c.apiKey)
	}
	req, _ := http.NewRequest("GET", tmdbBase+path+"?"+q.Encode(), nil)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		time.Sleep(2 * time.Second)
		return c.get(path, q, out)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d : %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}

type providersResp struct {
	Results []struct {
		ProviderID   int    `json:"provider_id"`
		ProviderName string `json:"provider_name"`
	} `json:"results"`
}

type discoverResp struct {
	TotalPages int `json:"total_pages"`
	Results    []struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		ReleaseDate string `json:"release_date"`
	} `json:"results"`
}

func (a *App) progress(msg string) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "progress", msg)
	}
}

// Generate interroge TMDB et renvoie la liste des films français disponibles
// sur les plateformes choisies, selon les types de monétisation choisis.
func (a *App) Generate(token string, useBearer bool, platforms []string, monetize []string, criteria string, enrich bool) GenerateResult {
	if strings.TrimSpace(token) == "" {
		return GenerateResult{Error: "Renseigne ta clé / ton jeton TMDB."}
	}
	if len(platforms) == 0 {
		return GenerateResult{Error: "Sélectionne au moins une plateforme."}
	}
	if len(monetize) == 0 {
		return GenerateResult{Error: "Sélectionne au moins un type de disponibilité."}
	}

	c := &tmdbClient{http: &http.Client{Timeout: 30 * time.Second}}
	if useBearer {
		c.bearer = strings.TrimSpace(token)
	} else {
		c.apiKey = strings.TrimSpace(token)
	}

	// 1. Liste des providers FR.
	a.progress("Récupération des plateformes TMDB (FR)…")
	var pr providersResp
	if err := c.get("/watch/providers/movie", url.Values{"watch_region": {"FR"}}, &pr); err != nil {
		return GenerateResult{Error: "Échec d'authentification / récupération des plateformes : " + err.Error()}
	}

	// 2. Résolution libellé -> IDs providers.
	type prov struct {
		id    int
		label string
	}
	var selected []prov
	for _, label := range platforms {
		subs, ok := providerMatch[label]
		if !ok {
			continue
		}
		for _, p := range pr.Results {
			name := strings.ToLower(p.ProviderName)
			for _, s := range subs {
				if strings.Contains(name, s) {
					selected = append(selected, prov{p.ProviderID, label})
					break
				}
			}
		}
	}
	if len(selected) == 0 {
		return GenerateResult{Error: "Aucune plateforme TMDB ne correspond à la sélection."}
	}

	mon := strings.Join(monetize, "|")
	films := map[int]*Film{}

	// 3. Parcours des pages discover par provider.
	for _, p := range selected {
		page, total := 1, 1
		for page <= total && page <= 500 {
			q := url.Values{
				"watch_region":                  {"FR"},
				"with_watch_providers":          {strconv.Itoa(p.id)},
				"with_watch_monetization_types": {mon},
				"language":                      {"fr-FR"},
				"sort_by":                       {"primary_release_date.desc"},
				"include_adult":                 {"false"},
				"without_genres":                {"99"}, // exclut les documentaires
				"page":                          {strconv.Itoa(page)},
			}
			switch criteria {
			case "language":
				q.Set("with_original_language", "fr")
			case "all":
				// Toutes nationalités : aucun filtre d'origine/langue.
				// Proxy « piste audio FR » : tout ce qui est dispo sur les plateformes FR.
			default: // "origin"
				q.Set("with_origin_country", "FR")
			}

			var dr discoverResp
			if err := c.get("/discover/movie", q, &dr); err != nil {
				return GenerateResult{Error: fmt.Sprintf("Erreur TMDB (%s, page %d) : %s", p.label, page, err.Error())}
			}
			total = dr.TotalPages
			if total > 500 {
				total = 500
			}
			for _, m := range dr.Results {
				f := films[m.ID]
				if f == nil {
					year := ""
					if len(m.ReleaseDate) >= 4 {
						year = m.ReleaseDate[:4]
					}
					f = &Film{TMDBID: m.ID, Title: m.Title, Year: year}
					films[m.ID] = f
				}
				if !contains(f.Platforms, p.label) {
					f.Platforms = append(f.Platforms, p.label)
				}
			}
			a.progress(fmt.Sprintf("%s : page %d/%d — %d films cumulés", p.label, page, max(total, 1), len(films)))
			page++
			time.Sleep(40 * time.Millisecond)
		}
	}

	// 4. Tri par titre.
	list := make([]Film, 0, len(films))
	for _, f := range films {
		sort.Strings(f.Platforms)
		list = append(list, *f)
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Title) < strings.ToLower(list[j].Title)
	})

	// 5. Enrichissement JustWatch (résolution + VF) — 1 requête par film, throttlé.
	if enrich {
		n := len(list)
		for i := range list {
			yr := 0
			if len(list[i].Year) >= 4 {
				yr, _ = strconv.Atoi(list[i].Year[:4])
			}
			res, vf := justwatchEnrich(list[i].Title, yr)
			list[i].Resolution = res
			list[i].VF = vf
			a.progress(fmt.Sprintf("Enrichissement JustWatch %d/%d — %s", i+1, n, list[i].Title))
			time.Sleep(150 * time.Millisecond) // ~6 req/s pour éviter le rate-limit
		}
	}

	a.progress(fmt.Sprintf("Terminé : %d films.", len(list)))
	return GenerateResult{Films: list, Count: len(list)}
}

// ExportCSV ouvre une boîte de dialogue et écrit le CSV (séparateur ';', BOM UTF-8 pour Excel FR).
func (a *App) ExportCSV(films []Film) (string, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultFilename: "films-fr.csv",
		Title:           "Enregistrer le CSV",
		Filters:         []wruntime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // annulé
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	f.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM UTF-8

	w := csv.NewWriter(f)
	w.Comma = ';'
	w.Write([]string{"tmdb_id", "lien_tmdb", "titre", "annee", "plateformes", "resolution_max", "vf"})
	for _, fl := range films {
		url := fmt.Sprintf("https://www.themoviedb.org/movie/%d", fl.TMDBID)
		w.Write([]string{strconv.Itoa(fl.TMDBID), url, fl.Title, fl.Year, strings.Join(fl.Platforms, ", "), fl.Resolution, fl.VF})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return path, nil
}

// ---------- Persistance des réglages ----------

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "GO-Films-FR")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func (a *App) SaveSettings(s Settings) error {
	p, err := settingsPath()
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

func (a *App) LoadSettings() Settings {
	s := Settings{
		UseBearer: true,
		Platforms: []string{"Netflix", "Prime Video", "Orange", "Canal+"},
		Monetize:  []string{"flatrate", "rent", "buy"},
		Criteria:  "origin",
		Enrich:    true,
	}
	p, err := settingsPath()
	if err != nil {
		return s
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return s
	}
	json.Unmarshal(b, &s)
	return s
}

// GetPlatforms renvoie la liste des plateformes proposées dans l'UI.
func (a *App) GetPlatforms() []string {
	return PlatformList
}

// ---------- helpers ----------

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
