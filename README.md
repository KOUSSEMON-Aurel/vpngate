# vpngate

Client pour [vpngate.net](https://www.vpngate.net/).

![vpngate](vpngate.gif)

Ce client récupère la liste des serveurs de relais fournis par vpngate.net, permet de les filtrer et de se connecter au serveur de son choix.

Particularité de ce fork : **chaque serveur est vérifié par un vrai probe OpenVPN avant d'être proposé**, car beaucoup de serveurs vpngate.net sont pleins ou en maintenance : ils répondent sur TCP/TLS mais rejettent l'authentification. Un simple test de ping TCP ne suffit donc pas à les détecter.

Vérifiez votre IP et votre région sur <https://ipinfo.io/json>, ou :

```shell
curl ipinfo.io
```

## Prérequis

- [OpenVPN](https://openvpn.net/)
- macOS, Linux, ou Windows

## Installation

### Homebrew (macOS et Linux)

```shell
brew install openvpn davegallant/public/vpngate
```

### Windows

Installez OpenVPN depuis le [site officiel](https://openvpn.net/community-downloads/), téléchargez l'archive de la release Windows et extrayez-la. Ouvrez ensuite une invite de commandes *en tant qu'administrateur* et lancez `vpngate.exe`.

### Compilation depuis les sources

Prérequis : [Go](https://go.dev/doc/install).

```shell
git clone https://github.com/KOUSSEMON-Aurel/vpngate.git
cd vpngate
go build -o vpngate .
```

Ou installez directement le binaire dans `$GOBIN` :

```shell
CGO_ENABLED=0 go install github.com/KOUSSEMON-Aurel/vpngate@latest
```

Assurez-vous que le chemin des binaires Go est accessible :

```shell
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.profile
source ~/.profile
```

## Utilisation

La référence complète des commandes est dans [la doc CLI](docs/cli/vpngate.md).

> Sur macOS (Homebrew), ajoutez openvpn au PATH : `export PATH=$(brew --prefix openvpn)/sbin:$PATH`

### Se connecter

```shell
# choix interactif dans le TUI, avec santé vérifiée en direct
sudo vpngate connect --country Japan

# connexion au serveur vérifié le plus rapide, sans prompt
sudo vpngate connect --best --country Japan

# serveur aléatoire avec filtres de qualité
sudo vpngate connect --random --country Japan --max-ping 100 --min-score 500000
```

> `openvpn` crée une interface réseau : lancez `connect` avec `sudo` ou un utilisateur ayant les droits élevés.

### Lister

```shell
# navigateur live (TUI) avec statut/latence/score de chaque serveur
vpngate list --tui --health-check

# serveurs japonais triés par ping le plus bas
vpngate list --country Japan --sort ping

# serveurs US à fort score en JSON
vpngate list --country us --min-score 1000000 --output json

# table simple, un seul passage de vérification, pas de TUI
vpngate list --tui=false --watch=false --health-check
```

### En arrière-plan

Démarrez la connexion en tâche de fond puis consultez-la, suivez son journal ou déconnectez-vous plus tard. `status`, `logs` et `disconnect` nécessitent aussi `sudo`, car l'état du daemon n'est lisible que par l'utilisateur qui a lancé `connect -d` (root) :

```shell
sudo vpngate connect -d --country Japan
sudo vpngate status
sudo vpngate logs -f
sudo vpngate disconnect
```

### Cache

```shell
vpngate cache path      # emplacement du cache
vpngate cache clear     # vide le cache
vpngate list --refresh  # rafraîchit la liste avant de lister
vpngate list --no-cache # ignore le cache
```

## Vérification réelle des serveurs

vpngate.net référence des centaines de serveurs, mais beaucoup sont pleins ou en maintenance : ils acceptent la connexion TCP/TLS puis rejettent l'authentification (`AUTH_FAILED`). Un contrôle de type « ping TCP » ne peut pas les distinguer d'un serveur sain.

Avant de connecter (ou de lister avec `--health-check`), vpngate lance donc un **vrai probe OpenVPN** sur chaque candidat en exécutant le binaire `openvpn`, et ne marque utilisable que le serveur qui complète la négociation (`PUSH_REPLY`). Les serveurs qui répondent `AUTH_FAILED` sont affichés comme inutilisables, et l'échec TCP/TLS est distingué (`unreachable`/`timeout`).

La vérification est **continue** : les résultats sont revérifiés en arrière-plan (toutes les `--watch-interval`, 30s par défaut) pendant que la liste reste affichée.

Comportement configurable :

| Option | Défaut | Description |
|---|---|---|
| `--health-check` | connect: true / list: false | probe OpenVPN réel avant sélection/listage |
| `--health-concurrency` | `10` | nombre de probes en parallèle |
| `--health-timeout` | `5s` | délai max par serveur |
| `--tui` | `true` | TUI interactif quand on est sur un terminal |
| `--watch` | `true` | revérification continue en arrière-plan |
| `--watch-interval` | `30s` | fréquence de revérification |

## Notes

- Je ne maintiens aucun des serveurs de vpngate.net (connexion à ces serveurs à vos risques et périls).
- Beaucoup de serveurs listés déclarent une politique de logs de 2 semaines.
