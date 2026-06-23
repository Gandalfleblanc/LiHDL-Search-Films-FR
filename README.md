# LiHDL Search Films FR

App desktop (Wails — Go + Svelte) qui liste les **films français** disponibles sur **Netflix / Prime Video / Orange / Canal+** en France, et exporte un **CSV**.

Données de disponibilité via **TMDB** (fournies par JustWatch) + enrichissement **JustWatch** pour la **résolution** (HD / 4K) et la présence d'une **piste audio FR (VF)**.

## Fonctionnalités

- Découverte des films via l'API TMDB (`/discover/movie`, région FR)
- 3 critères « film français » : pays d'origine FR, langue VO française, ou toutes nationalités
- Disponibilité : abonnement / location / achat
- Enrichissement JustWatch (optionnel) : **résolution max** (4K > HD > SD) + **VF** (oui / non / inconnu)
- Export CSV : `tmdb_id ; titre ; annee ; plateformes ; resolution_max ; vf`

## Configuration

Une **clé / un jeton API TMDB** (gratuit, sur themoviedb.org) est requis — à coller dans l'app (persisté localement).

## Build local

```bash
~/go/bin/wails build
```

## Releases

Les binaires (macOS arm64/x64, Linux, Windows) sont publiés automatiquement via GitHub Actions à chaque tag `vX.Y.Z`. Voir la page [Releases](../../releases).
