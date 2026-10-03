# Modules IA locaux Actelyo

Cette distribution ajoute trois modules optionnels à Actelyo : **Legal Inference** (moteur et modèles GGUF), **Legal Harness** (agent et outils) et **LLMQushu** (documents et RAG).

## Préparer et lancer sur votre PC

Prérequis : Python 3.11+, Git, Docker Desktop démarré sur Windows/macOS, ou Docker Engine avec Compose v2 sous Linux. Les conteneurs utilisent Linux. Prévoyez l'espace des images, des compilations et des modèles ; le téléchargement d'un modèle n'est pas inclus.

Extrayez l'archive dans un dossier local hors des dossiers synchronisés, puis ouvrez un terminal dans ce dossier :

```text
python actelyo-local.py prepare
python actelyo-local.py config
python actelyo-local.py up
python actelyo-local.py status
```

Pour un seul module : `python actelyo-local.py prepare inference`, puis `python actelyo-local.py up inference`. Les autres choix sont `harness` et `llmqushu`.

`prepare` récupère les versions exactes indiquées dans `sources.json`, applique les modifications Actelyo fournies et crée un fichier `.env` contenant des secrets propres à l'installation. Une seconde exécution conserve les secrets et ne réapplique pas une modification déjà présente. Un checkout divergent est refusé et conservé. Le premier `up` construit les images : une connexion internet est nécessaire.

Les interfaces sont disponibles uniquement sur votre PC :

| Module | Interface | API d'inférence |
| --- | --- | --- |
| Legal Inference | http://127.0.0.1:8091 | http://127.0.0.1:8092/v1 |
| Legal Harness | http://127.0.0.1:9119 | Protocole de l'agent |
| LLMQushu | http://127.0.0.1:3001 | API du module documentaire |

Le compte du Harness est `actelyo`. Son mot de passe est la valeur `ACTELYO_HARNESS_PASSWORD` du fichier `.env`. Chaque module conserve son propre compte et ses autorisations : cette version n'ajoute pas de connexion unique avec le compte Actelyo.

## Configurer un véritable modèle

Legal Inference démarre son interface sans prétendre qu'un modèle est chargé. Dans l'interface, installez le moteur llama.cpp, téléchargez ou fournissez un modèle GGUF compatible avec votre matériel, puis choisissez le modèle et démarrez le moteur. La variante conteneur gère directement son processus de moteur, sans systemd. La configuration CPU fonctionne sans accès GPU ; une variante GPU nécessite ses pilotes et une image adaptée et n'est pas fournie ici.

Dans Actelyo, ouvrez **Paramètres → Modules IA locaux**, indiquez l'identifiant exact du modèle chargé puis choisissez **Vérifier et connecter**. La connexion est enregistrée seulement si l'API expose ce modèle. Un refus du navigateur, un moteur arrêté ou un modèle absent reste un échec visible. Cette connexion utilise le moteur local sans clé ; si vous protégez son API avec une clé, configurez également le client correspondant avant de l'utiliser.

Pour Harness et LLMQushu, un conteneur atteint l'inférence via `http://legal-inference:8080/v1` (le `127.0.0.1` d'un conteneur désigne ce conteneur). Configurez le fournisseur OpenAI compatible et le nom du modèle dans chaque module. Dans LLMQushu, configurez aussi le fournisseur d'embeddings et créez un espace documentaire. Pour le Harness, configurez votre fournisseur via son interface ou ses commandes. Aucun corpus, modèle juridique entraîné ou modèle de fournisseur n'est fourni ni renommé par cette distribution.

L'ouverture d'Actelyo en HTTPS peut limiter l'accès au localhost HTTP suivant le navigateur. Utilisez l'installation locale d'Actelyo pour connecter le moteur ; ouvrir les trois interfaces dans leurs propres onglets reste possible. Le lanceur ajoute les modules à un Actelyo existant ; il ne remplace pas les prérequis et la configuration de la base de données de l'ERP.

## Arrêter, consulter les erreurs et conserver les données

```text
python actelyo-local.py logs
python actelyo-local.py stop
```

`stop` arrête seulement les modules sélectionnés et conserve leurs volumes. Ne supprimez pas les volumes avant sauvegarde. `status` vérifie uniquement la disponibilité HTTP de l'interface, pas la réponse d'un modèle. Les identifiants générés dans `.env` et les volumes doivent être sauvegardés séparément.

## Branding et composants tiers

Les interfaces de ces variantes portent les noms et le logo Actelyo. Les licences MIT, les copyrights d'origine, les notices des composants tiers, l'historique Git et les identifiants techniques nécessaires à la compatibilité sont conservés. Les noms de fournisseurs et modèles restent exacts ; ils ne deviennent pas des produits Actelyo. Les services distants tiers optionnels présents dans les moteurs d'origine ne sont pas des services Actelyo et ne sont pas nécessaires au déploiement local décrit ici.
