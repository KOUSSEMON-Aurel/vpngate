# Guide de Publication & Conformité Google Play Console — OpenRelay VPN

Ce document détaille toutes les étapes, les textes exacts à copier/coller, les déclarations obligatoires et les réponses aux questionnaires de la Google Play Console pour assurer une validation rapide et sans rejet d'**OpenRelay VPN**.

---

## 1. Fiche Play Store (Store Listing)

### Nom de l'application (Titre)
> **OpenRelay VPN - Secure Client**  
*(Moins de 30 caractères, conforme à la politique d'évitement d'usurpation d'identité).*

### Description courte (Short Description - max 80 caractères)
> **Client VPN open source rapide et sécurisé propulsé par OpenVPN et WireGuard.**

### Description complète (Full Description)
```text
OpenRelay VPN est un client VPN moderne, léger et entièrement open source conçu pour protéger votre vie privée et sécuriser vos connexions en ligne.

Grâce à OpenRelay VPN, connectez-vous facilement aux serveurs de relais publics bénévoles du projet académique VPN Gate (Université de Tsukuba) ou utilisez notre relais Anycast haute performance propulsé par la technologie WireGuard.

CARACTÉRISTIQUES PRINCIPALES :
• Double protocole : Prise en charge complète d'OpenVPN et de WireGuard.
• Carte mondiale dynamique : Visualisez l'emplacement et la latence des serveurs relais en temps réel.
• Relais Anycast ultra-rapide : Connexion instantanée sans latence grâce au protocole WireGuard.
• Aucun compte requis : Pas d'inscription, pas d'e-mail, pas de mot de passe ni de carte bancaire.
• Protection DNS intégrée : Prévention des fuites DNS et chiffrement de bout en bout du trafic.
• Transparence totale : Code source ouvert sous licence GPLv2 / MIT.

AVERTISSEMENT ACADÉMIQUE ET LÉGAL :
OpenRelay VPN est un client indépendant développé par la communauté open source. Cette application n'est ni affiliée, ni approuvée, ni sponsorisée par l'Université de Tsukuba ou le projet VPN Gate. Les serveurs relais sont hébergés par des bénévoles à travers le monde dans le cadre d'un programme de recherche académique sur les réseaux.

CONFIDENTIALITÉ :
L'application OpenRelay VPN ne collecte, n'enregistre et ne revend aucune donnée personnelle ni historique de navigation. Tout le trafic réseau est chiffré de bout en bout via l'API Android VpnService.
```

---

## 2. Déclaration Obligatoire VpnService (Politique Google Play)

Google exige une justification spécifique et une vidéo de démonstration pour toute application utilisant la permission `android.permission.BIND_VPN_SERVICE`.

### Champ : Fonctionnalité principale (Core Feature Justification)
Copiez-collez ce texte dans le formulaire de déclaration VpnService de la console :
```text
OpenRelay VPN is a dedicated VPN client application whose core and primary functionality is to establish a secure, encrypted virtual private network tunnel. 

The Android VpnService API is strictly required to route device network traffic through OpenVPN and WireGuard tunnels, encrypting user packets and protecting user privacy against network surveillance and unencrypted public Wi-Fi risks. The VPN connection is only initiated upon explicit user consent via the connect button following a clear in-app disclosure dialog.
```

### Vidéo de démonstration YouTube (Requis par Google Play)
Google exige un lien YouTube non répertorié (*Unlisted*) d'environ 30 à 60 secondes montrant :
1. **L'ouverture de l'application** montrant l'écran d'accueil d'OpenRelay VPN.
2. **Le clic sur le bouton "CONNECT"** : faire apparaître la boîte de dialogue de divulgation bien visible (*Prominent Disclosure Dialog*).
3. **Le clic sur "I UNDERSTAND & AGREE"** : la demande de permission système standard d'Android s'ouvre.
4. **La connexion réussie** : le statut passe à `CONNECTED` en vert et l'icône de clé VPN apparaît dans la barre de statut d'Android.
5. **Le panneau de notification déroulé** : montrer la notification du service de premier plan affichant les statistiques réseau.
6. **Le clic sur "DISCONNECT"** pour arrêter la session.

---

## 3. Formulaire Sécurité des Données (Data Safety Section)

Remplissez le questionnaire Play Console avec les réponses suivantes :

| Question Play Console | Réponse à cocher | Justification |
|---|---|---|
| **L'application collecte-t-elle ou partage-t-elle des données utilisateur ?** | **NON** (No) | L'application ne conserve aucune donnée sur des serveurs propriétaires. |
| **Toutes les données utilisateur collectées par l'application sont-elles chiffrées en transit ?** | **OUI** (Yes) | Tout le trafic réseau passe par un tunnel chiffré OpenVPN ou WireGuard. |
| **Permettez-vous aux utilisateurs de demander la suppression de leurs données ?** | **NON APPLICABLE** (ou Oui) | Aucun compte utilisateur n'existe, aucune donnée n'est stockée. |
| **Données de localisation ?** | **NON collectées** | La carte affiche la position des serveurs publics, pas celle du téléphone. |
| **Identifiants personnels (Nom, Email, Téléphone) ?** | **NON collectés** | Aucun compte requis. |
| **Données financières ou de paiement ?** | **NON collectées** | Application 100% gratuite. |

---

## 4. Politique de Confidentialité (Privacy Policy)

Google exige un lien public vers une Politique de Confidentialité valide.  
Lien recommandé à renseigner dans la Play Console :
`https://github.com/KOUSSEMON-Aurel/vpngate/blob/mobile/PRIVACY_POLICY.md`

### Texte officiel de la Politique de Confidentialité (Inclus dans l'application et sur GitHub) :
Le fichier `PRIVACY_POLICY.md` a été généré à la racine du dépôt GitHub et est directement consultable.

---

## 5. Services au Premier Plan (Foreground Service Types - Android 14+)

Dans la déclaration des autorisations de services de premier plan (FGS) de la Play Console :
- **Types déclarés dans le Manifest** : `systemExempted` et `specialUse`.
- **Cas d'usage à sélectionner** : Maintien continu de la connexion VPN et affichage du flux de trafic réseau sécurisé dans la barre d'état.

---

## 6. Conformité Propriété Intellectuelle & Licences

1. **OpenVPN 3 Core** : Déclaré sous licence **GPLv2**. La clause de redistribution est respectée par la mise à disposition publique du code source sur GitHub.
2. **WireGuard Go** : Déclaré sous licence **Apache 2.0**.
3. **Absence d'usurpation de marque** : Aucune référence aux marques tierces commerciales dans le titre ou les captures d'écran du store.
4. **Attribution académique** : La clause d'indépendance avec l'Université de Tsukuba figure dans la description du Store et sur l'écran Paramètres de l'application.
