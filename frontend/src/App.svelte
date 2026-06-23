<script>
  import { onMount } from 'svelte'
  import { Generate, ExportCSV, SaveSettings, LoadSettings, GetPlatforms } from '../wailsjs/go/main/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import logo from './assets/images/lihdl-logo.png'
  import banner from './assets/images/banner.png'

  let token = ''
  let useBearer = true
  let allPlatforms = ['Netflix', 'Prime Video', 'Orange', 'Canal+']
  let platforms = { Netflix: true, 'Prime Video': true, Orange: true, 'Canal+': true }
  let monetize = { flatrate: true, rent: true, buy: true }
  let criteria = 'origin' // 'origin' | 'language' | 'all'
  let enrich = true // enrichissement JustWatch (résolution + VF)

  let loading = false
  let progressMsg = ''
  let errorMsg = ''
  let films = []
  let count = 0
  let exportInfo = ''
  let filter = ''

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

    EventsOn('progress', (msg) => (progressMsg = msg))
  })

  function selectedPlatforms() {
    return allPlatforms.filter((p) => platforms[p])
  }
  function selectedMonetize() {
    return ['flatrate', 'rent', 'buy'].filter((m) => monetize[m])
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
    })

    try {
      const res = await Generate(token, useBearer, selectedPlatforms(), selectedMonetize(), criteria, enrich)
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

  $: shown = filter
    ? films.filter(
        (f) =>
          f.title.toLowerCase().includes(filter.toLowerCase()) ||
          ('' + f.tmdb_id).includes(filter)
      )
    : films
</script>

<div class="bg" style="background-image: url({banner})"></div>

<main>
  <header>
    <div class="logo" style="background-image: url({logo})"></div>
    <div class="titles">
      <h1>LiHDL Search Films FR</h1>
      <p class="sub">Films français disponibles sur les plateformes de streaming (données TMDB / JustWatch)</p>
    </div>
  </header>

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
        <span class="legend">Enrichissement (JustWatch)</span>
        <div class="chips">
          <label class="chip" class:on={enrich}><input type="checkbox" bind:checked={enrich} />Résolution + VF (plus lent)</label>
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
                <td class="num">{f.tmdb_id}</td>
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
  .year { width: 64px; color: #b9c6d8; }
  .plats { color: #9fd0b4; }
  .res { width: 72px; }
  .vf { width: 52px; text-align: center; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 6px; font-size: 0.78rem; font-weight: 600; }
  .res-4k { background: #4a2d6b; color: #d9b8ff; }
  .res-hd { background: #1d3a5f; color: #aacdf5; }
  .res-sd { background: #3a3f47; color: #c2c8d0; }
</style>
