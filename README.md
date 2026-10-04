# ACTELYO INFERENCE HUB

Gestionnaire local de LLM, intégré à ACTELYO ERP : moteur llama.cpp,
modèles GGUF, réglages GPU et contexte, presets, chat, mémoire, outils,
serveurs MCP et API compatible OpenAI.

## Compiler et démarrer

Go est requis selon la version indiquée dans go.mod.

```sh
git clone https://github.com/jeanneretsamy-ux/actelyo-inference-hub.git
cd actelyo-inference-hub
go generate ./internal/actelyohub
go build -o actelyohub ./cmd/actelyohub
./actelyohub install
./actelyohub llamacpp install
./actelyohub edit
./actelyohub set-web-key
./actelyohub start
./actelyohub web
```

Sur Windows, compiler avec `go build -o actelyohub.exe ./cmd/actelyohub`.
Renseigner BIN (llama-server) et MODEL (fichier GGUF) dans la configuration.
La compilation de llama.cpp nécessite CMake et un compilateur compatible
avec la machine ; un binaire déjà installé peut aussi être utilisé.

Le hub écoute sur `http://127.0.0.1:8090` par défaut. Le modèle ne répond
qu'après installation effective du moteur et chargement d'un GGUF adapté
au matériel. Aucun modèle n'est livré dans le dépôt.

## Utiliser depuis ACTELYO ERP

Ouvrir **Intelligence artificielle → ACTELYO INFERENCE HUB**
(`/inference-hub`), saisir l'adresse et la clé de pilotage. La page lit
l'état et la configuration réels et commande démarrage, arrêt et redémarrage.
Elle ouvre l'interface complète du hub pour modifier les presets, choisir
les modèles, régler le moteur et utiliser le chat et ses outils.

La clé de pilotage reste en mémoire dans la page ERP. L'interface complète
du hub demande sa propre authentification. La clé d'inférence OpenAI est
distincte de la clé de pilotage.

Cloudflare Pages héberge le frontend ERP ; le hub Go doit tourner sur
l'ordinateur local ou un serveur CPU/GPU. Pour un serveur distant, utiliser
un reverse proxy HTTPS et configurer la clé de pilotage avant exposition.
Le navigateur peut demander une autorisation d'accès au réseau local.

| Variable | Usage |
| --- | --- |
| ACTELYO_HUB_HOME | Répertoire des données du hub |
| ACTELYO_HUB_BIND_HOST | Adresse d'écoute web, défaut 127.0.0.1 |
| ACTELYO_HUB_ALLOWED_ORIGINS | Origines ERP autorisées, séparées par des virgules ; défaut https://actelyo.com,https://www.actelyo.com |
| ACTELYO_HUB_CATALOG_URL | Catalogue distant facultatif ; sinon catalogue embarqué |
| ACTELYO_HUB_LINK_URL | Relais compatible facultatif ; aucun relais par défaut |

Le relais distant, les sauvegardes externes et les adresses publiques de
relais ne sont pas provisionnés. Leurs commandes sont conservées pour
un déploiement compatible explicite ; les contrôles correspondants sont
masqués dans l'interface. Les domaines `.example` dans le code de relais
sont des réservations de développement, sans service opérationnel.

Le fork utilise ses propres noms de services, répertoires et variables.
Une installation existante nécessite une sauvegarde et une migration
explicite de ses données ; aucune migration de données personnelles n'est
exécutée par ces changements de code.

## Vérifier

```sh
go vet ./...
go build ./...
go test ./...
```

Les workflows de release publient les binaires `actelyohub-*` lors d'un tag.
Les anciens binaires de l'amont ne sont pas des releases de ce fork.

## Licence

MIT. La notice originale est conservée dans LICENSE.
