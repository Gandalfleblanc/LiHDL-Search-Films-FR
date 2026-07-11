package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Client JustWatch (GraphQL non officiel) — sert à récupérer la résolution
// (SD/HD/4K) et les langues audio (présence VF), que TMDB n'expose pas.

const jwEndpoint = "https://apis.justwatch.com/graphql"

const jwQuery = `query($filter: TitleFilter!, $country: Country!, $language: Language!, $first: Int!) { popularTitles(country: $country, filter: $filter, first: $first) { edges { node { objectType content(country: $country, language: $language) { title originalReleaseYear } offers(country: $country, platform: WEB) { monetizationType presentationType audioLanguages package { clearName } } } } } }`

// Libellé plateforme (app) -> sous-chaînes du nom de package JustWatch.
// NB : JustWatch FR ne référence PAS Orange -> résolution/VF « inconnu » pour Orange.
var jwPackageMatch = map[string][]string{
	"Netflix":     {"netflix"},
	"Prime Video": {"amazon prime video"},
	"Orange":      {"orange"},
	"Canal+":      {"canal"},
}

var jwClient = &http.Client{Timeout: 20 * time.Second}

type jwReq struct {
	Variables map[string]any `json:"variables"`
	Query     string         `json:"query"`
}

type jwResp struct {
	Data struct {
		PopularTitles struct {
			Edges []struct {
				Node struct {
					ObjectType string `json:"objectType"`
					Content    struct {
						Title               string `json:"title"`
						OriginalReleaseYear int    `json:"originalReleaseYear"`
					} `json:"content"`
					Offers []struct {
						MonetizationType string   `json:"monetizationType"`
						PresentationType string   `json:"presentationType"`
						AudioLanguages   []string `json:"audioLanguages"`
						Package          struct {
							ClearName string `json:"clearName"`
						} `json:"package"`
					} `json:"offers"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"popularTitles"`
	} `json:"data"`
}

// justwatchEnrich cherche un film par titre+année (région FR) et renvoie la
// résolution + VF calculées UNIQUEMENT sur les offres des plateformes et types
// de monétisation sélectionnés (sinon on mélangerait les résolutions d'autres
// plateformes — cf. bug « HD affiché alors que SD sur Orange »).
//   - resolution : "4K" / "HD" / "SD" / "" (introuvable / plateforme non couverte)
//   - vf         : "oui" / "non" / "inconnu"
func justwatchEnrich(title string, year int, platforms, monetize []string) (resolution, vf string) {
	resolution, vf = "", "inconnu"

	// Sous-chaînes de packages JustWatch à retenir selon les plateformes choisies.
	var pkgSubs []string
	for _, label := range platforms {
		pkgSubs = append(pkgSubs, jwPackageMatch[label]...)
	}
	// Types de monétisation retenus (JustWatch : FLATRATE / RENT / BUY).
	monSet := map[string]bool{}
	for _, m := range monetize {
		monSet[strings.ToUpper(m)] = true
	}

	body, err := json.Marshal(jwReq{
		Variables: map[string]any{
			"country":  "FR",
			"language": "fr",
			"first":    4,
			"filter":   map[string]any{"searchQuery": title},
		},
		Query: jwQuery,
	})
	if err != nil {
		return
	}

	req, _ := http.NewRequest("POST", jwEndpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := jwClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		time.Sleep(2 * time.Second)
		return justwatchEnrich(title, year, platforms, monetize)
	}
	if resp.StatusCode != http.StatusOK {
		return
	}

	var r jwResp
	if json.NewDecoder(resp.Body).Decode(&r) != nil {
		return
	}

	// Choisit le meilleur film : priorité au match d'année (±1), sinon 1er film.
	best := -1
	for i, e := range r.Data.PopularTitles.Edges {
		if e.Node.ObjectType != "MOVIE" {
			continue
		}
		if year > 0 && abs(e.Node.Content.OriginalReleaseYear-year) <= 1 {
			best = i
			break
		}
		if best == -1 {
			best = i
		}
	}
	if best == -1 {
		return
	}
	offers := r.Data.PopularTitles.Edges[best].Node.Offers

	rank, anyAudio, hasFr := 0, false, false
	for _, o := range offers {
		// Ne garder que les offres des plateformes sélectionnées…
		if len(pkgSubs) > 0 {
			name := strings.ToLower(o.Package.ClearName)
			matched := false
			for _, s := range pkgSubs {
				if strings.Contains(name, s) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		// …et des types de monétisation sélectionnés.
		if len(monSet) > 0 && !monSet[o.MonetizationType] {
			continue
		}
		switch o.PresentationType {
		case "_4K":
			if rank < 3 {
				rank = 3
			}
		case "HD":
			if rank < 2 {
				rank = 2
			}
		case "SD":
			if rank < 1 {
				rank = 1
			}
		}
		if len(o.AudioLanguages) > 0 {
			anyAudio = true
			for _, l := range o.AudioLanguages {
				if l == "fr" {
					hasFr = true
				}
			}
		}
	}

	switch rank {
	case 3:
		resolution = "4K"
	case 2:
		resolution = "HD"
	case 1:
		resolution = "SD"
	}
	switch {
	case hasFr:
		vf = "oui"
	case anyAudio:
		vf = "non"
	default:
		vf = "inconnu"
	}
	return
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
