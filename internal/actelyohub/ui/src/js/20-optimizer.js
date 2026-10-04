let optimizerPollTimer = null;
let optimizerActionPending = false;
function renderOptimizer(report) {
  const running = report.state === 'running';
  document.getElementById('optimizer-run').disabled = running || !!report.can_restore || optimizerActionPending;
  document.getElementById('optimizer-objective').disabled = running || optimizerActionPending;
  document.getElementById('optimizer-apply').hidden = report.state !== 'completed' || report.recommended < 0 || report.applied;
  document.getElementById('optimizer-restore').hidden = !report.can_restore;
  document.getElementById('optimizer-cancel').hidden = !running;
  for (const action of ['apply', 'restore', 'cancel']) document.getElementById('optimizer-'+action).disabled = optimizerActionPending;
  document.getElementById('optimizer-progress').textContent = report.error || report.progress || 'Prêt à comparer les profils du modèle chargé.';
  const rows = (report.profiles || []).map((p,i) => `${p.name}${i === report.recommended ? ' ✓ recommandé' : ''} : température ${p.temperature} · ${p.passed}/6 vérifications · ${p.tokens_per_second.toFixed(1)} tokens/s`);
  if (report.context) rows.unshift(`Modèle : ${report.model}\nContexte réel : ${report.context} tokens · concurrence ${report.parallel}\nRAM libre avant tests : ${report.free_ram_mb} Mo\n`);
  document.getElementById('optimizer-results').textContent = rows.join('\n');
  document.getElementById('optimizer-warning').textContent = report.warning || '';
  clearTimeout(optimizerPollTimer);
  if (running && document.getElementById('optimizer-panel').open) optimizerPollTimer = setTimeout(refreshOptimizer, 3000);
}
async function refreshOptimizer() {
  try { renderOptimizer(await jget('/api/optimizer')); }
  catch (err) { document.getElementById('optimizer-progress').textContent = 'Suivi interrompu : '+err.message; }
}
async function optimizerAction(action) {
  if (optimizerActionPending) return;
  optimizerActionPending = true;
  for (const name of ['run','apply','restore','cancel']) document.getElementById('optimizer-'+name).disabled = true;
  document.getElementById('optimizer-progress').textContent = 'Action en cours…';
  try {
    const payload = action === 'run' ? {objective:document.getElementById('optimizer-objective').value} : {};
    const result = await jpost('/api/optimizer/'+action, payload);
    if (result.ok === false) throw new Error(result.error || 'Action refusée');
    optimizerActionPending = false;
    await refreshOptimizer();
    if (action === 'apply' || action === 'restore') await loadCfg();
  } catch (err) {
    optimizerActionPending = false;
    await refreshOptimizer();
    document.getElementById('optimizer-progress').textContent = 'Action impossible : '+err.message;
  }
}
