# ACTELYO INFERENCE HUB 0.15.0

Première version ACTELYO : interface, binaire `actelyohub`, services,
répertoires, variables et mises à jour sous la nouvelle marque.

Intégration avec la page `/inference-hub` d'ACTELYO ERP : lecture du statut,
configuration et commandes de démarrage/arrêt/redémarrage authentifiées.
Clé de pilotage distincte de la clé d'inférence, origines ERP autorisées
explicitement, écoute locale par défaut.

Les modèles, réglages GPU/contexte, presets, chat, mémoire, outils et MCP
restent disponibles dans l'interface complète. Aucun modèle GGUF n'est
inclus : installer llama-server et un modèle adapté au matériel.

Le relais externe n'est pas provisionné et n'est plus contacté par défaut.
Les identifiants de services et chemins de données changent : sauvegarder
et migrer explicitement toute ancienne installation avant utilisation.
La notice MIT originale est conservée.
