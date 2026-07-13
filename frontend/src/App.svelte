<script>
  import { onMount } from 'svelte'
  import { Generate, ExportCSV, SaveSettings, LoadSettings, GetPlatforms, LoadExclusionCSV, ClearExclusion, GetVersion, CheckUpdate, DoUpdate } from '../wailsjs/go/main/App.js'
  import { EventsOn, BrowserOpenURL } from '../wailsjs/runtime/runtime.js'
  import logo from './assets/images/lihdl-logo.png'
  import banner from './assets/images/banner.png'

  let token = ''
  let useBearer = true
  let allPlatforms = ['Netflix', 'Prime Video', 'Orange', 'Canal+']
  let platforms = { Netflix: true, 'Prime Video': true, Orange: true, 'Canal+': true }
  let monetize = { flatrate: true, rent: true, buy: true }
  let criteria = 'origin' // 'origin' | 'language' | 'all'
  let enrich = true // enrichissement JustWatch (résolution + VF)
  let exclude = { documentaire: true, telefilm: true, court: true, spectacle: true }

  let loading = false
  let progressMsg = ''
  let errorMsg = ''
  let films = []
  let count = 0
  let exportInfo = ''
  let filter = ''
  let sortBy = 'titre' // 'titre' | 'resolution' | 'annee'
  let exclCount = 0
  let exclFile = ''
  let version = ''
  let update = { available: false, latest: '', url: '', notes: '' }
  let updating = false

  onMount(async () => {
    allPlatforms = await GetPlatforms()
    const s = await LoadSettings()
    token = s.token || ''
    useBearer = s.useBearer
    const sel = new Set(s.platforms || allPlatforms)
    platforms = {}
    for (const p of allPlatforms) platforms[p] = sel.has(p)
    const mon = new Set(s.monetize || ['flatrate', 'rent', 'buy'])
    monetize = { flatrate: mon.has('flatrate'), rent: mon.has('rent'), buy: mon.has('buy') }
    criteria = s.criteria || 'origin'
    enrich = s.enrich !== false
    const ex = new Set(s.exclude || ['documentaire', 'telefilm', 'court', 'spectacle'])
    exclude = {
      documentaire: ex.has('documentaire'),
      telefilm: ex.has('telefilm'),
      court: ex.has('court'),
      spectacle: ex.has('spectacle'),
    }

    EventsOn('progress', (msg) => (progressMsg = msg))

    version = await GetVersion()
    CheckUpdate().then((u) => {
      if (u && !u.error) update = u
    })
  })

  async function installUpdate() {
    updating = true
    progressMsg = 'Mise à jour en cours…'
    const r = await DoUpdate()
    if (r && r.error) {
      errorMsg = r.error
      updating = false
    }
    // Sur macOS, l'app se ferme et redémarre toute seule.
  }

  function openTmdb(id) {
    BrowserOpenURL(`https://www.themoviedb.org/movie/${id}`)
  }

  async function loadExclusion() {
    const r = await LoadExclusionCSV()
    if (r.error) {
      errorMsg = 'Exclusion : ' + r.error
      return
    }
    exclCount = r.count
    if (r.file) exclFile = r.file
  }

  async function clearExclusion() {
    await ClearExclusion()
    exclCount = 0
    exclFile = ''
  }

  function selectedPlatforms() {
    return allPlatforms.filter((p) => platforms[p])
  }
  function selectedMonetize() {
    return ['flatrate', 'rent', 'buy'].filter((m) => monetize[m])
  }
  function selectedExclude() {
    return ['documentaire', 'telefilm', 'court', 'spectacle'].filter((e) => exclude[e])
  }

  async function run() {
    errorMsg = ''
    exportInfo = ''
    films = []
    count = 0
    loading = true
    progressMsg = 'Démarrage…'

    await SaveSettings({
      token,
      useBearer,
      platforms: selectedPlatforms(),
      monetize: selectedMonetize(),
      criteria,
      enrich,
      exclude: selectedExclude(),
    })

    try {
      const res = await Generate(token, useBearer, selectedPlatforms(), selectedMonetize(), criteria, enrich, selectedExclude())
      if (res.error) {
        errorMsg = res.error
      } else {
        films = res.films || []
        count = res.count
      }
    } catch (e) {
      errorMsg = '' + e
    } finally {
      loading = false
    }
  }

  async function exportCsv() {
    exportInfo = ''
    try {
      const path = await ExportCSV(films)
      exportInfo = path ? 'Enregistré : ' + path : 'Export annulé.'
    } catch (e) {
      exportInfo = 'Erreur export : ' + e
    }
  }

  const resRank = { '4K': 3, HD: 2, SD: 1 }

  $: shown = (() => {
    let list = filter
      ? films.filter(
          (f) =>
            f.title.toLowerCase().includes(filter.toLowerCase()) ||
            ('' + f.tmdb_id).includes(filter)
        )
      : films.slice()
    if (sortBy === 'resolution') {
      list = list.slice().sort((a, b) => (resRank[b.resolution] || 0) - (resRank[a.resolution] || 0))
    } else if (sortBy === 'annee') {
      list = list.slice().sort((a, b) => (b.year || '').localeCompare(a.year || ''))
    } else {
      list = list.slice().sort((a, b) => a.title.toLowerCase().localeCompare(b.title.toLowerCase()))
    }
    return list
  })()
</script>

<div class="bg" style="background-image: url({banner})"></div>

<main>
  <header>
    <div class="logo" style="background-image: url({logo})"></div>
    <div class="titles">
      <h1>LiHDL Search Films FR <span class="ver">v{version}</span></h1>
      <p class="sub">Films français disponibles sur les plateformes de streaming (données TMDB / JustWatch)</p>
    </div>
  </header>

  {#if update.available}
    <div class="update-banner">
      <span>🎉 Mise à jour <strong>v{update.latest}</strong> disponible (tu as v{version}).</span>
      <div class="update-actions">
        {#if update.url}<button class="btn-link" on:click={() => BrowserOpenURL(update.url)}>Notes de version</button>{/if}
        <button class="btn primary" on:click={installUpdate} disabled={updating}>
          {updating ? '⏳ Installation…' : '⬇️ Installer et redémarrer'}
        </button>
      </div>
    </div>
  {/if}

  <section class="card">
    <div class="row">
      <label class="field grow">
        <span>Clé / Jeton TMDB</span>
        <input type="password" bind:value={token} placeholder="Jeton v4 (Bearer) ou clé v3" autocomplete="off" />
      </label>
      <label class="field auth">
        <span>Type</span>
        <select bind:value={useBearer}>
          <option value={true}>Jeton v4 (Bearer)</option>
          <option value={false}>Clé v3</option>
        </select>
      </label>
    </div>

    <div class="row groups">
      <div class="group">
        <span class="legend">Plateformes</span>
        <div class="chips">
          {#each allPlatforms as p}
            <label class="chip" class:on={platforms[p]}>
              <input type="checkbox" bind:checked={platforms[p]} />{p}
            </label>
          {/each}
        </div>
      </div>

      <div class="group">
        <span class="legend">Disponibilité</span>
        <div class="chips">
          <label class="chip" class:on={monetize.flatrate}><input type="checkbox" bind:checked={monetize.flatrate} />Abonnement</label>
          <label class="chip" class:on={monetize.rent}><input type="checkbox" bind:checked={monetize.rent} />Location</label>
          <label class="chip" class:on={monetize.buy}><input type="checkbox" bind:checked={monetize.buy} />Achat</label>
        </div>
      </div>

      <div class="group">
        <span class="legend">Critère « film français »</span>
        <div class="chips">
          <label class="chip" class:on={criteria === 'origin'}><input type="radio" bind:group={criteria} value="origin" />Pays d'origine FR</label>
          <label class="chip" class:on={criteria === 'language'}><input type="radio" bind:group={criteria} value="language" />Langue VO française</label>
          <label class="chip" class:on={criteria === 'all'}><input type="radio" bind:group={criteria} value="all" />Toutes nationalités (audio FR probable)</label>
        </div>
      </div>

      <div class="group">
        <span class="legend">Exclure du résultat</span>
        <div class="chips">
          <label class="chip" class:on={exclude.documentaire}><input type="checkbox" bind:checked={exclude.documentaire} />Documentaires</label>
          <label class="chip" class:on={exclude.telefilm}><input type="checkbox" bind:checked={exclude.telefilm} />Téléfilms</label>
          <label class="chip" class:on={exclude.court}><input type="checkbox" bind:checked={exclude.court} />Courts métrages (&lt; 40 min)</label>
          <label class="chip" class:on={exclude.spectacle}><input type="checkbox" bind:checked={exclude.spectacle} />Spectacles / concerts</label>
        </div>
      </div>

      <div class="group">
        <span class="legend">Enrichissement (JustWatch)</span>
        <div class="chips">
          <label class="chip" class:on={enrich}><input type="checkbox" bind:checked={enrich} />Résolution + VF (plus lent)</label>
        </div>
      </div>

      <div class="group">
        <span class="legend">Exclure des films (CSV de n° TMDB)</span>
        <div class="chips">
          <button class="chip btn-chip" on:click={loadExclusion}>📄 Charger un CSV</button>
          {#if exclCount}
            <span class="chip on excl-info">{exclCount} exclus{exclFile ? ` · ${exclFile}` : ''}</span>
            <button class="chip btn-chip" on:click={clearExclusion} title="Retirer la liste d'exclusion">✕</button>
          {/if}
        </div>
      </div>
    </div>

    <div class="actions">
      <button class="btn primary" on:click={run} disabled={loading}>
        {loading ? '⏳ Génération…' : '🔎 Générer la liste'}
      </button>
      <button class="btn" on:click={exportCsv} disabled={!films.length || loading}>💾 Exporter CSV</button>
      {#if count}<span class="count">{count} films</span>{/if}
    </div>

    {#if loading || progressMsg}
      <div class="status">{progressMsg}</div>
    {/if}
    {#if errorMsg}<div class="status error">{errorMsg}</div>{/if}
    {#if exportInfo}<div class="status ok">{exportInfo}</div>{/if}
  </section>

  {#if films.length}
    <section class="card results">
      <div class="results-head">
        <input class="search" placeholder="Filtrer (titre ou n° TMDB)…" bind:value={filter} />
        <label class="sort">Trier&nbsp;:
          <select bind:value={sortBy}>
            <option value="titre">Titre</option>
            <option value="resolution">Résolution (4K→SD)</option>
            <option value="annee">Année (récent)</option>
          </select>
        </label>
        <span class="muted">{shown.length} affichés</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th class="num">TMDB</th><th>Titre</th><th class="year">Année</th><th>Plateformes</th><th class="res">Résol.</th><th class="vf">VF</th></tr>
          </thead>
          <tbody>
            {#each shown as f (f.tmdb_id)}
              <tr>
                <td class="num"><button class="tmdb-link" on:click={() => openTmdb(f.tmdb_id)} title="Ouvrir la fiche TMDB">{f.tmdb_id}</button></td>
                <td>{f.title}</td>
                <td class="year">{f.year}</td>
                <td class="plats">{f.platforms.join(', ')}</td>
                <td class="res"><span class="badge res-{(f.resolution || '').toLowerCase()}">{f.resolution || '—'}</span></td>
                <td class="vf"><span class="badge vf-{f.vf || ''}">{f.vf === 'oui' ? '✅' : f.vf === 'non' ? '❌' : f.vf === 'inconnu' ? '❓' : '—'}</span></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}
</main>

<style>
  :global(html) { background: #07090d; }
  .bg {
    position: fixed;
    inset: 0;
    z-index: -1;
    background-size: cover;
    background-position: center top;
    background-repeat: no-repeat;
  }
  /* voile sombre dégradé pour garder le texte lisible */
  .bg::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(180deg, rgba(7, 9, 13, 0.55) 0%, rgba(7, 9, 13, 0.78) 45%, rgba(7, 9, 13, 0.9) 100%);
  }
  main {
    max-width: 1000px;
    margin: 0 auto;
    padding: 22px 26px 40px;
    text-align: left;
    color: #e8edf4;
  }
  header { display: flex; align-items: center; gap: 16px; margin-bottom: 18px; }
  .logo { width: 56px; height: 56px; border-radius: 12px; background-size: cover; background-position: center; background-repeat: no-repeat; flex-shrink: 0; box-shadow: 0 2px 10px rgba(0,0,0,.4); }
  .titles { display: flex; flex-direction: column; }
  header h1 { margin: 0 0 2px; font-size: 1.5rem; }
  .ver { font-size: 0.7rem; color: #8da2bd; font-weight: 400; vertical-align: middle; background: #1b2735; padding: 2px 7px; border-radius: 6px; }
  .update-banner {
    display: flex; align-items: center; justify-content: space-between; gap: 14px;
    background: linear-gradient(90deg, #1d3a5f, #2f6fd1); border: 1px solid #4b86d6;
    border-radius: 10px; padding: 12px 16px; margin-bottom: 16px; font-size: 0.9rem;
  }
  .update-actions { display: flex; align-items: center; gap: 10px; }
  .btn-link { background: none; border: none; color: #cfe0f7; text-decoration: underline; cursor: pointer; font: inherit; }
  .btn-link:hover { color: #fff; }
  .sub { margin: 0; color: #8da2bd; font-size: 0.85rem; }

  .card {
    background: rgba(20, 29, 41, 0.82);
    backdrop-filter: blur(6px);
    border: 1px solid #2a3a4e;
    border-radius: 12px;
    padding: 18px 20px;
    margin-bottom: 18px;
  }

  .row { display: flex; gap: 14px; align-items: flex-end; }
  .row.groups { margin-top: 16px; flex-wrap: wrap; }
  .field { display: flex; flex-direction: column; gap: 5px; }
  .field span { font-size: 0.72rem; color: #8da2bd; text-transform: uppercase; letter-spacing: .04em; }
  .grow { flex: 1; }
  input[type='password'], select, .search {
    background: #0f1722;
    border: 1px solid #2c3c52;
    color: #e8edf4;
    border-radius: 8px;
    padding: 9px 11px;
    font-size: 0.9rem;
    outline: none;
  }
  input[type='password']:focus, select:focus, .search:focus { border-color: #4b86d6; }

  .group { display: flex; flex-direction: column; gap: 7px; }
  .legend { font-size: 0.72rem; color: #8da2bd; text-transform: uppercase; letter-spacing: .04em; }
  .chips { display: flex; gap: 8px; flex-wrap: wrap; }
  .chip {
    display: inline-flex; align-items: center; gap: 6px;
    background: #0f1722; border: 1px solid #2c3c52;
    border-radius: 999px; padding: 6px 12px;
    font-size: 0.85rem; cursor: pointer; user-select: none;
    transition: all .12s ease;
  }
  .chip.on { background: #1d3a5f; border-color: #4b86d6; color: #cfe0f7; }
  .chip input { accent-color: #4b86d6; }
  .btn-chip { font: inherit; color: #e8edf4; }
  .btn-chip:hover { background: #1f2c3d; border-color: #4b86d6; }
  .excl-info { cursor: default; }

  .actions { display: flex; align-items: center; gap: 12px; margin-top: 20px; }
  .btn {
    border: 1px solid #2c3c52; background: #1f2c3d; color: #e8edf4;
    border-radius: 8px; padding: 10px 16px; font-size: 0.9rem; cursor: pointer;
    transition: all .12s ease;
  }
  .btn:hover:not(:disabled) { background: #26374c; }
  .btn:disabled { opacity: .5; cursor: not-allowed; }
  .btn.primary { background: #2f6fd1; border-color: #2f6fd1; font-weight: 600; }
  .btn.primary:hover:not(:disabled) { background: #3a7ee0; }
  .count { color: #7fd0a0; font-weight: 600; }

  .status { margin-top: 14px; font-size: 0.85rem; color: #9fb4d0; min-height: 1.1em; }
  .status.error { color: #ff8b8b; }
  .status.ok { color: #7fd0a0; }

  .results-head { display: flex; align-items: center; gap: 14px; margin-bottom: 12px; }
  .search { flex: 1; }
  .sort { display: flex; align-items: center; gap: 6px; color: #8da2bd; font-size: 0.82rem; white-space: nowrap; }
  .sort select { background: #0f1722; border: 1px solid #2c3c52; color: #e8edf4; border-radius: 8px; padding: 7px 9px; font-size: 0.85rem; }
  .muted { color: #8da2bd; font-size: 0.82rem; }
  .table-wrap { max-height: 50vh; overflow: auto; border-radius: 8px; border: 1px solid #243244; }
  table { width: 100%; border-collapse: collapse; font-size: 0.86rem; }
  thead th {
    position: sticky; top: 0; background: #1b2735; color: #9fb4d0;
    text-align: left; padding: 9px 12px; font-weight: 600; border-bottom: 1px solid #243244;
  }
  tbody td { padding: 8px 12px; border-bottom: 1px solid #1c2836; }
  tbody tr:hover { background: #1b2735; }
  .num { color: #7fa8dd; font-variant-numeric: tabular-nums; width: 80px; }
  .tmdb-link {
    background: none; border: none; padding: 0; cursor: pointer;
    color: #7fa8dd; font: inherit; font-variant-numeric: tabular-nums;
    text-decoration: underline; text-decoration-color: rgba(127, 168, 221, 0.4);
  }
  .tmdb-link:hover { color: #aacdf5; text-decoration-color: #aacdf5; }
  .year { width: 64px; color: #b9c6d8; }
  .plats { color: #9fd0b4; }
  .res { width: 72px; }
  .vf { width: 52px; text-align: center; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 6px; font-size: 0.78rem; font-weight: 600; }
  .res-4k { background: #4a2d6b; color: #d9b8ff; }
  .res-hd { background: #1d3a5f; color: #aacdf5; }
  .res-sd { background: #3a3f47; color: #c2c8d0; }
</style>
