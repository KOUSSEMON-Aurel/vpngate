#!/usr/bin/env bash
set -euo pipefail

# Racine du dépôt
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# 1. Extraction de la version actuelle
GRADLE_FILE="android/app/build.gradle.kts"
TAURI_FILE="gui/src-tauri/tauri.conf.json"
PKG_FILE="gui/package.json"

if [ ! -f "$GRADLE_FILE" ]; then
  echo "❌ Erreur: Impossible de trouver $GRADLE_FILE"
  exit 1
fi

CURRENT_VERSION=$(grep -E 'versionName\s*=\s*' "$GRADLE_FILE" | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/')
CURRENT_CODE=$(grep -E 'versionCode\s*=\s*' "$GRADLE_FILE" | head -n 1 | sed -E 's/.*=\s*([0-9]+).*/\1/')

if [ -z "$CURRENT_VERSION" ] || [ -z "$CURRENT_CODE" ]; then
  echo "❌ Erreur: Impossible de lire versionName ou versionCode dans $GRADLE_FILE"
  exit 1
fi

# 2. Calcul des versions suivantes
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"
NEXT_PATCH="${MAJOR}.${MINOR}.$((PATCH + 1))"
NEXT_MINOR="${MAJOR}.$((MINOR + 1)).0"
NEXT_MAJOR="$((MAJOR + 1)).0.0"
NEXT_CODE=$((CURRENT_CODE + 1))

MODE="${1:-}"

# Si aucun argument n'est fourni, mode interactif
if [ -z "$MODE" ]; then
  echo ""
  echo "=========================================================="
  echo " 📦 Version actuelle : v${CURRENT_VERSION} (Android versionCode: ${CURRENT_CODE})"
  echo "=========================================================="
  echo " ❓ Voulez-vous publier une nouvelle version pour ce push ?"
  echo "   1) Push normal (aucun changement de version)"
  echo "   2) Patch  (v${CURRENT_VERSION} ➔ v${NEXT_PATCH}  | Android code: ${NEXT_CODE})"
  echo "   3) Minor  (v${CURRENT_VERSION} ➔ v${NEXT_MINOR}  | Android code: ${NEXT_CODE})"
  echo "   4) Major  (v${CURRENT_VERSION} ➔ v${NEXT_MAJOR}  | Android code: ${NEXT_CODE})"
  echo "   5) Version personnalisée"
  echo "   q) Annuler"
  echo "----------------------------------------------------------"

  # Lecture interactive
  if [ -t 0 ]; then
    read -p "Votre choix [1/2/3/4/5/q] : " -r CHOICE
  elif [ -r /dev/tty ]; then
    read -p "Votre choix [1/2/3/4/5/q] : " -r CHOICE < /dev/tty
  else
    echo "Pas de terminal interactif détecté. Utilisation du mode normal."
    CHOICE="1"
  fi
else
  CHOICE="$MODE"
fi

DO_BUMP=false
NEW_VER=""
NEW_CODE="$NEXT_CODE"

case "$CHOICE" in
  1|normal|none|skip)
    echo "🚀 Exécution d'un push normal (aucun changement de version)..."
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    git push origin "$CURRENT_BRANCH"
    echo "✅ Push terminé avec succès sur '$CURRENT_BRANCH'."
    exit 0
    ;;
  2|patch)
    NEW_VER="$NEXT_PATCH"
    DO_BUMP=true
    ;;
  3|minor)
    NEW_VER="$NEXT_MINOR"
    DO_BUMP=true
    ;;
  4|major)
    NEW_VER="$NEXT_MAJOR"
    DO_BUMP=true
    ;;
  5|custom)
    if [ -t 0 ]; then
      read -p "Entrez la nouvelle version (ex: 1.2.0) : " -r CUSTOM_VER
    elif [ -r /dev/tty ]; then
      read -p "Entrez la nouvelle version (ex: 1.2.0) : " -r CUSTOM_VER < /dev/tty
    else
      echo "Erreur: Mode interactif requis pour version personnalisée."
      exit 1
    fi
    NEW_VER="${CUSTOM_VER#v}" # Enlève le v initial si tapé
    DO_BUMP=true
    ;;
  q|quit|cancel)
    echo "⏹️ Push annulé."
    exit 0
    ;;
  *)
    # Si l'argument passé directement est un numéro de version (ex: ./tools/push.sh 1.0.3)
    if [[ "$CHOICE" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      NEW_VER="${CHOICE#v}"
      DO_BUMP=true
    else
      echo "❌ Option invalide : '$CHOICE'."
      exit 1
    fi
    ;;
esac

if [ "$DO_BUMP" = true ]; then
  echo ""
  echo "🔧 Application de la version v${NEW_VER} (Android versionCode: ${NEW_CODE})..."

  # 1. Android build.gradle.kts
  sed -i -E "s/(versionCode\s*=\s*)[0-9]+/\1${NEW_CODE}/" "$GRADLE_FILE"
  sed -i -E "s/(versionName\s*=\s*\")[^\"]+(\")/\1${NEW_VER}\2/" "$GRADLE_FILE"

  # 2. Tauri Desktop GUI
  if [ -f "$TAURI_FILE" ]; then
    sed -i -E "s/(\"version\"\s*:\s*\")[^\"]+(\")/\1${NEW_VER}\2/" "$TAURI_FILE"
  fi
  if [ -f "$PKG_FILE" ]; then
    sed -i -E "s/(\"version\"\s*:\s*\")[^\"]+(\")/\1${NEW_VER}\2/" "$PKG_FILE"
  fi

  # 3. Documentation & Site Web
  for doc in docs/index.html docs/features.html docs/privacy.html docs/terms.html; do
    if [ -f "$doc" ]; then
      sed -i -E "s/v[0-9]+\.[0-9]+\.[0-9]+/v${NEW_VER}/g" "$doc"
    fi
  done

  # Vérification
  echo "✓ Android : versionName=${NEW_VER}, versionCode=${NEW_CODE}"
  echo "✓ Desktop GUI : version=${NEW_VER}"
  echo "✓ Site Web : liens mis à jour vers v${NEW_VER}"

  # Mode test / dry-run si spécifié en second argument
  if [ "${2:-}" = "--dry-run" ] || [ "${2:-}" = "--test" ]; then
    echo "🧪 Mode test terminé (aucun commit ni push n'a été effectué)."
    exit 0
  fi

  # 4. Commit & Tag Git
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  git add "$GRADLE_FILE" "$TAURI_FILE" "$PKG_FILE" docs/
  git commit -m "chore(release): bump version to v${NEW_VER}"
  git tag -a "v${NEW_VER}" -m "Release v${NEW_VER}"

  echo "🚀 Envoi sur GitHub (commits de la branche '$CURRENT_BRANCH' + tag v${NEW_VER})..."
  git push origin "$CURRENT_BRANCH" --tags

  echo ""
  echo "=========================================================="
  echo "🎉 Version v${NEW_VER} publiée avec succès !"
  echo "Le workflow GitHub Actions 'Unified Multi-Platform Release'"
  echo "est en cours d'exécution pour compiler vos binaires."
  echo "=========================================================="
fi
