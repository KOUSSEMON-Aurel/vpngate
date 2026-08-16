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

### Script d'installation (Linux et macOS)

```shell
curl -fsSL https://raw.githubusercontent.com/KOUSSEMON-Aurel/vpngate/main/install.sh | bash
```

Le script installe le binaire (release GitHub, sinon `go install`), puis les
dépendances détectées par le gestionnaire de paquets (apt, dnf, pacman, brew) :
`openvpn`, `wireguard-tools`, et `wgcf` quand Go est disponible. Il ne fait
jamais échouer l'installation si une dépendance optionnelle manque.

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
>
> **Alternative sans sudo (Linux)** : donnez à openvpn la seule capability nécessaire, une fois pour toutes, puis lancez `connect` normalement (le TUI fonctionne aussi sans sudo) :
>
> ```shell
> sudo setcap cap_net_admin+ep /usr/bin/openvpn
> vpngate connect --country Japan
> ```
>
> Si votre système utilise systemd-resolved, la restauration du DNS par openvpn déclenche un prompt polkit à chaque connexion/déconnexion. Autorisez-la une fois pour toutes, puis **redémarrez polkit** (les règles ne sont lues qu'au démarrage) :
>
> ```shell
> sudo mkdir -p /etc/polkit-1/rules.d
> sudo tee /etc/polkit-1/rules.d/50-resolvconf.rules <<'EOF'
> polkit.addRule(function(action, subject) {
>     if (action.id == "org.freedesktop.resolve1.revert" ||
>         action.id == "org.freedesktop.resolve1.set-dns-servers" ||
>         action.id == "org.freedesktop.resolve1.set-domains" ||
>         action.id == "org.freedesktop.resolve1.set-dnssec" ||
>         action.id == "org.freedesktop.resolve1.set-dns-over-tls" ||
>         action.id == "org.freedesktop.resolve1.set-link-llmnr" ||
>         action.id == "org.freedesktop.resolve1.set-link-mdns") {
>         return polkit.Result.YES;
>     }
> });
> EOF
> sudo systemctl restart polkit
> ```

### Se connecter à Cloudflare WARP

```shell
sudo vpngate warp
```

WARP utilise **wgcf** (enregistrement de compte + profil WireGuard, amené
avec `wg-quick`) quand il est installé, sinon **warp-cli** en repli.

- Avec le backend wgcf : `wgcf` et `wg-quick` doivent être installés ; le
  profil est créé automatiquement dans `~/.config/wgcf/wgcf-profile.conf`
  (chemin surchargeable avec `--wgcf-config`). Nécessite `sudo` (interface tun).
- Avec le backend warp-cli : `warp-cli` doit être connecté au préalable
  (`warp-cli register --accept-tos && warp-cli connect`).

### Lister

```shell
# navigateur live (TUI) avec statut/latence/score de chaque serveur
vpngate list --tui --health-check

# serveurs japonais triés par ping le plus bas
vpngate list --country Japan --sort ping

# serveurs US à fort score en JSON
vpngate list --country us --min-score 1000000 --output json

# uniquement les serveurs d'une source donnée (vpngate, vpnbook, warp)
vpngate list --source warp

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

## Garde-fou du tunnel connecté

Une fois connecté, vpngate surveille le tunnel vivant et le **répare ou le laisse tranquille** intelligemment. Les relais vpngate.net sont communautaires et leur sortie (egress) est souvent partielle ou instable — un seul endpoint de santé provoquait des faux négatifs (tunnel tué à tort → chaîne de reconnexion → « all 8 attempted relays failed »).

Le check de santé est donc **multi-endpoints** : `www.gstatic.com/generate_204`, `www.google.com/generate_204`, `https://1.1.1.1/cdn-cgi/trace` et `https://8.8.8.8/` sont sondés en parallèle, et **une seule réponse HTTP quelconque** (n'importe quel code) suffit à considérer le tunnel vivant. Deux endpoints sont en IP pure (pas de dépendance au DNS du tunnel).

Le watchdog n'est pas brutal :

- **Fenêtre de grâce** : aucun check pendant les 30 premières secondes après connexion.
- **Seuil de 5 échecs consécutifs** (toutes les 10 s) avant de couper et de passer au relais suivant : un à-coup d'egress ne fait plus tomber une connexion saine.
- Seul un relais dont **tous** les endpoints échouent en continu (~80 s) est déclaré mort et remplacé.

### Mettre le garde-fou en pause

Pendant la connexion, appuyez sur **`p`** dans le TUI : le footer passe de `[p] health on` à `[p] health off` et le watchdog cesse de sonder — **une connexion vivante n'est alors jamais coupée par vpngate**, même sur un relais à l'egress chaotique. Re-appuyez sur `p` pour reprendre la surveillance.

En ligne de commande, le garde-fou se désactive complètement avec `--tunnel-health-check=false` :

```shell
sudo vpngate connect --tunnel-health-check=false
```

Comportement configurable (sondes toutes les 10 s, grâce de 30 s, seuil de 5 échecs — valeurs internes) :

| Option | Défaut | Description |
|---|---|---|
| `--tunnel-health-check` | `true` | surveille le tunnel une fois connecté (sondes HTTPS multi-endpoints) |

## Idées futures (TODO)

- [x] **WARP comme source dans le TUI** : Cloudflare WARP (`Source: warp`) est fusionné dans la liste. WARP n'a pas de relais communautaire à vérifier : son probe est un simple marqueur « working » sans latence mesurée, il trie donc *en dernier* dans les tris par ping (et derrière tous les relais mesurés dans `--best` et les reconnexions), tout en restant connectable explicitement (`connect warp`). Connecté via `wg-quick` (wgcf, création du profil automatique) ou `warp-cli` si disponible.
- [x] **Installeur one-liner** : `curl -fsSL https://raw.githubusercontent.com/KOUSSEMON-Aurel/vpngate/main/install.sh | bash` installant openvpn, wireguard-tools, wgcf (et éventuellement warp-cli), puis le binaire vpngate.
- [ ] **vpnbook multi-transport** : exposer les transports vpnbook `tcp443`/`tcp80`/`udp53`/`udp25000` (aujourd'hui un seul profil `tcp443` par serveur pour qu'il n'apparaisse qu'une fois dans la liste).
- [ ] **`container-as-gateway` + kill switch** : router tout le trafic de la machine via un conteneur OpenVPN (passerelle par défaut = conteneur), avec règles nftables/iptables bloquant toute sortie hors du tunnel (`DROP` sauf via `tun0`), IPv6 bloqué et DNS forcé dans le tunnel → **zéro fuite** (IP et DNS) même si le tunnel tombe. Docker seul en `--network host` n'isole ni ne reroute rien : il partage le réseau de l'hôte.

## Notes

- Je ne maintiens aucun des serveurs de vpngate.net (connexion à ces serveurs à vos risques et périls).
- Beaucoup de serveurs listés déclarent une politique de logs de 2 semaines.
