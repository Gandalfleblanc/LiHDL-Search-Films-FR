package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// Client JustWatch (GraphQL non officiel) — sert à récupérer la résolution
// (SD/HD/4K) et les langues audio (présence VF), que TMDB n'expose pas.

const jwEndpoint = "https://apis.justwatch.com/graphql"

const jwQuery = `query($filter: TitleFilter!, $country: Country!, $language: Language!, $first: Int!) { popularTitles(country: $country, filter: $filter, first: $first) { edges { node { objectType content(country: $country, language: $language) { title originalReleaseYear } offers(country: $country, platform: WEB) { presentationType audioLanguages } } } } }`

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
						PresentationType string   `json:"presentationType"`
						AudioLanguages   []string `json:"audioLanguages"`
					} `json:"offers"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"popularTitles"`
	} `json:"data"`
}

// justwatchEnrich cherche un film par titre+année (région FR) et renvoie :
//   - resolution : "4K" / "HD" / "SD" / "" (introuvable)
//   - vf         : "oui" / "non" / "inconnu"
func justwatchEnrich(title string, year int) (resolution, vf string) {
	resolution, vf = "", "inconnu"

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
		return justwatchEnrich(title, year)
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
