# Pokélecture

Apprendre à lire en français (méthode syllabique) avec les Pokémon. Serveur Go +
frontend statique, tout tourne en local sur le réseau de la maison, accessible
depuis une tablette.

---

## Démarrage

```bash
make content            # valide le contenu, génère la voix, télécharge les sprites
make run NAME=Prénom    # affiche les URLs du réseau local
```

Puis sur la tablette : ouvrir `http://192.168.x.x:8080` (l'adresse est affichée
au démarrage) → menu Chrome → **Ajouter à l'écran d'accueil**.

La sauvegarde est un simple fichier JSON : `var/progress.json`. `make reset`
repart de zéro.

---

## Le principe

**Tout le vocabulaire est du vocabulaire Pokémon** : noms de Pokémon, noms
d'attaques, types, personnages, objets. Pas de « lama » ni de « tomate ».

Le problème, c'est que les noms français de Pokémon ne sont pas déchiffrables
avec l'alphabet d'un débutant. La solution n'est pas de renoncer au thème :
c'est de **choisir l'ordre des graphèmes pour débloquer du vocabulaire Pokémon le
plus vite possible**.

Exemple, la route 1 : `Otaria` ne contient que `o-t-a-r-i-a`. En ajoutant `tt` et
`ss` dès le premier épisode, il lit aussi `Rattata`, `Ramoloss` et `Amonita`,
l'attaque `Morsure`, le type `Normal` — et la vraie phrase de combat
**« Otaria utilise Morsure. »**, dès le premier soir.

Conséquences assumées de ce choix :

- La route 1 introduit 14 graphèmes d'un coup (`a i o u é e m l r t s n tt ss`).
  C'est beaucoup, mais il connaît déjà les lettres : cet épisode est un
  échauffement/diagnostic, et les cartes se regroupent en une galerie à
  balayer plutôt qu'en 14 écrans.
- `oo` est enseigné comme un vrai graphème « spécial Pokémon » (`Rou-cool`,
  `Tenta-cool`), parce que c'est comme ça que ces noms s'écrivent.
- L'ordre s'écarte de la progression CP classique. C'est délibéré.

La séance reste structurée comme un CP : son → fusion de syllabes → mots →
phrases → combat. Un épisode ≈ **10–12 min**, une fois par jour.

### Ce qui est atteignable en un mois

À 10–15 min/jour : il fusionne **toutes les syllabes CV**, lit **~50 noms
Pokémon, attaques et types**, et des phrases de combat complètes. Excellent
résultat.

La lecture *fluide* prend une année de CP entière. Aucune app ne change ça — le
planning ci-dessous vise l'objectif atteignable, la progression continue après.

---

## Architecture

```
cmd/pokelecture      serveur HTTP (stdlib, zéro dépendance)
cmd/pokecontent      CLI de contenu : check | audio | sprites
internal/curriculum  chargement + VALIDATION du contenu, génération des séances
internal/progress    sauvegarde JSON + répétition espacée (5 boîtes)
internal/speech      texte → clé de fichier audio stable
data/                le contenu (c'est ici que tu passeras ton temps)
web/                 frontend : HTML/CSS/JS, aucun build
web/audio/           .m4a générés — **versionnés** (voir Déploiement)
web/sprites/         sprites téléchargés (régénérables, non versionnés)
```

**Décision clé** : un épisode *déclare* du contenu, il ne scripte pas d'écrans.
`BuildSession` génère la liste d'activités, mélange le matériel et injecte la
révision espacée. Ajouter un épisode = écrire un fichier JSON, pas du code.

**Pas de build frontend, volontairement** : un changement de contenu à 22h00 est
visible sur la tablette à 22h01.

### Le filet de sécurité qui compte

`pokecontent check` segmente chaque mot et chaque phrase avec **uniquement les
graphèmes déjà enseignés à ce stade** (plus long match d'abord, pour que `ou`
gagne sur `o`+`u`). Un mot non déchiffrable fait échouer la validation, et le
serveur refuse de démarrer.

Il vérifie aussi que le découpage syllabique recompose le mot, que chaque mot a
une image (emoji **ou** Pokémon), et que les Pokémon référencés existent.

---

## Écrire un épisode

`data/episodes/ep10.json` :

```json
{
  "id": "ep10",
  "route": 10,
  "title": "Route 10 — le son en",
  "story": "...",
  "new_graphemes": ["en"],
  "syllables": ["ben", "den", "len", "men", "pen", "ren", "sen", "ten"],
  "words": [
    {"text": "Tentacool", "pokemon": 72, "syllables": ["Ten", "ta", "cool"]},
    {"text": "Vent",      "emoji": "🌬️", "syllables": ["Vent"]}
  ],
  "sentences": [
    {"text": "Tentacool utilise Cascade.", "pokemon": 72}
  ],
  "reward_pokemon": 72,
  "boss": {"pokemon": 72, "hp": 3, "taunt": "Un Tentacool flotte devant toi !"}
}
```

`pokemon: 72` → le mot est illustré par le sprite de Tentacool. Sinon `emoji`.
Tout Pokémon cité doit exister dans `data/pokemon.json` (nom français, découpage
syllabique, types, texte de Pokédex écrit à la main en français simple).

Puis : `make content`.

### Les images : jamais d'emoji pour l'abstrait

Un mot est illustré par **un sprite de Pokémon** (`"pokemon": 86`), **une icône
qu'on fournit** (`"icon": "bite"`) ou, à défaut, un emoji.

Les emoji ont été retirés de tout ce qui est abstrait, pour deux raisons
mesurées sur l'appareil :

- un terme comme le type « Normal » ou la stat « Vitesse » n'a pas d'emoji qui
  veuille dire quoi que ce soit à 5 ans — ⚪ et 💨 ressemblent à une case vide ;
- les emoji récents n'existent pas dans la police de l'appareil : 🪽 et 🪺
  demandent Android 13+, 🪨 demande Android 11+. En dessous : case blanche.

Les icônes sont du SVG inline dans `web/icons.js`, aux couleurs officielles des
types. `curriculum.Icons` (Go) liste les noms valides : une faute de frappe fait
échouer `pokecontent check` au lieu d'afficher une case vide à l'enfant.

### La prononciation : `say_as`

Aucun TTS ne dit correctement « Pikachu », « Roucool » ou « Goupix ». Un
`say_as` donne l'orthographe phonétique **pour la voix seulement**, l'enfant
continuant à voir la vraie orthographe :

```json
{"id": 25, "name": "Pikachu", "say_as": "Pikatchou", ...}
{"id": 16, "name": "Roucool", "say_as": "Roukoul",   ...}
```

Ça marche aussi sur un `word` ou une `sentence`. Et si ça ne suffit pas :
enregistre-le au studio voix, c'est définitif.

### Arènes légendaires

Un épisode avec `"legendary": true` ne déclare **aucun** contenu : il tire ses
syllabes, ses mots et ses phrases de **tout ce qui a été appris avant**. C'est
donc à la fois une vraie révision cumulée et le moment où un Pokémon légendaire
entre au Pokédex. Le combat est plus long (5–6 PV, 6 mots) et l'écran passe en
violet et or.

Livrées : **Artikodin** après la route 3, **Électhor** après la route 6,
**Mewtwo** après la route 9. En réserve dans `pokemon.json` pour les sections
suivantes : Sulfura, Lugia, Ho-Oh, Celebi, Mew, Zoroark — ils apparaissent en
silhouette au Pokédex, ce qui donne envie d'aller les chercher.

### Règles de contenu, apprises à la dure

- **Pas de lettre finale muette** : `Aspicot` se lit /aspiko/, il lirait
  /aspikot/. Pareil pour `Pistolet`, `Coup`, `Vent`… Le validateur ne peut pas
  l'attraper, c'est à toi d'y penser. Les noms sûrs sont ceux dont la dernière
  lettre se prononce (`Otaria`, `Ramoloss`, `Goupix`, `Machoc`).
- **`en` est un piège** : dès qu'il est enseigné, la segmentation gloutonne
  transforme `Chenipan` en `ch-en-i-pan` (/ʃɑ̃ipɑ̃/). C'est pour ça que `an` et
  `en` sont dans deux épisodes séparés, et que `Chenipan` arrive **avant** `en`.
  Même piège pour `Menu`, `Cendre`.
- **`c` devant `e`/`i` fait /s/** : `Carapuce`, `Lance-Flammes` ne sont pas
  déchiffrables tant que `ce`/`ci` ne sont pas enseignés comme graphèmes.
- **`g` devant `e`/`i` fait /ʒ/** : d'où les graphèmes distincts `ge` et `gi`
  (`Charge`, `Magicarpe`).
- **Varie les verbes.** Deux phrases « X utilise Y. » d'affilée se lisent comme
  un seul exercice répété, et il arrête de lire le milieu de la phrase.
  `pokecontent check` refuse maintenant un épisode dont deux phrases partagent le
  même verbe. Verbes déchiffrables tôt : utilise, imite, adore, salue, vole,
  évite, roule, cache, monte, tombe, danse, bouge.

---

## Studio voix — enregistrer ta propre voix

Aucun TTS ne prononce correctement « Roucool », « Miaouss » ou « Salamèche », et
les sons de consonnes isolés sont son point faible absolu. La voix de synthèse
n'est donc qu'un **filet de sécurité** : la vraie voix du jeu, c'est la tienne.

```bash
make run
# puis, sur l'ORDINATEUR : http://localhost:8080/parent.html
```

Maintiens **ENREGISTRER** pendant que tu parles, relâche : c'est envoyé. Le
fichier est servi à la place de la synthèse **à la même URL**, donc le jeu n'a
rien à savoir. **↺** revient à la synthèse.

Les lignes sont classées **dans l'ordre de la progression** : les 39 sons
d'abord (route 1 en tête), puis syllabes, mots, phrases. Enregistrer juste les
sons, c'est ~5 minutes et c'est là que se joue l'essentiel.

> Le micro exige un contexte sécurisé : ouvre la page sur `localhost`, pas via
> l'adresse IP du réseau. Les enregistrements vont dans `web/audio/recorded/` et
> `pokecontent prune` n'y touche jamais.

### Pourquoi les `say_as` sont ce qu'ils sont

Un `say_as` doit être **une seule émission de voix**. Les formes allongées
(`"mmmmm"`, `"chchchch"`) et les séries (`"ta, te, ti, to, tu"`) sont lues par le
TTS comme **plusieurs sons répétés** — exactement ce qu'il ne faut pas faire
entendre à un enfant qui apprend un son.

D'où la règle : voyelles et digraphes = le signe seul (`a`, `ou`, `an`) ;
consonnes = la syllabe avec e muet (`me`, `te`, `che`), qui est la façon
française standard de faire sonner une consonne isolée en une seule fois.

---

## Choix techniques et pourquoi

| Choix | Raison |
|---|---|
| Audio pré-généré, pas de TTS temps réel | latence nulle, voix constante, marche hors-ligne |
| Un enregistrement écrase la synthèse à la même URL | le jeu ignore l'existence du studio voix : rien à câbler côté client |
| Fallback `speechSynthesis` si le fichier manque | un ajout de contenu ne casse jamais le jeu |
| Sprite pour les mots Pokémon, emoji pour le reste | l'image doit donner le sens sans lire |
| Écran « COMMENCER » | Chrome refuse de jouer du son avant un geste utilisateur |
| Pas de micro / reconnaissance vocale | ~40–60 % d'erreur sur une voix d'enfant : se faire dire « non » à tort est le plus sûr moyen de le dégoûter. Il lit à voix haute, puis touche « 🔊 Vérifier » pour s'auto-corriger. |
| Fichier JSON, pas de base de données | un seul enfant, et un fichier lisible aide quand tu debugges |
| Icônes SVG maison pour l'abstrait | l'emoji dépend de la police de l'appareil : sur Android ancien, 🪽 et 🪺 sont des cases vides |
| Bruitages synthétisés (Web Audio) | zéro fichier à télécharger, et le son est ce qui fait « jeu » à 5 ans |

### Répétition espacée

Chaque syllabe et chaque mot est un *item* avec 5 boîtes
(immédiat / 10 min / 1 j / 3 j / 7 j). Une bonne réponse monte d'une boîte, une
erreur en descend **d'une seule** (un faux contact sur tablette ne doit pas
effacer un vrai acquis). Chaque séance commence par les 5 items dus les plus
fragiles des épisodes précédents.

---

## Le thème

Palette officielle (`#ee1515` rouge, `#3b4cca` bleu, `#ffcb05` jaune), Poké Ball
en CSS pur réutilisée comme logo / spinner / marqueur de progression, barre du
haut en verre dépoli dessinée comme la coque d'un Pokédex, artworks officiels
plutôt que sprites pixel, badges aux couleurs officielles des types, arène de
combat avec barre de PV verte→jaune→rouge, et un compagnon (Pikachu au départ,
puis sa dernière capture) qui le suit d'écran en écran et saute à chaque bonne
réponse.

Le son fait le reste : `web/sfx.js` synthétise tout au Web Audio, sans un seul
fichier — carillon montant sur une bonne réponse, whoosh d'attaque, arpège de
capture, grondement à l'entrée d'un légendaire. Plus des confettis sur une
capture et en fin de séance.

---

## Épisodes livrés (routes 1 à 9)

| Route | Sons | Vocabulaire Pokémon | Récompense |
|---|---|---|---|
| 1 | a i o u é e m l r t s n tt ss | Otaria, Rattata, Ramoloss, Amonita, Morsure, Normal | Rattata |
| 2 | p d | Doduo, Dodrio, Nidorina, Paras, Tornade | Doduo |
| 3 | f v | Mélofée, Évoli, Nosferapti, Fée, Vol, Vitesse | Évoli |
| 4 | b c | Abo, Sabelette, Caninos, Cascade, Croc | Caninos |
| 5 | ou oo | Roucool, Miaouss, Poudre, Roc, Coupe | Roucool |
| 6 | ch è k | **Pikachu**, Salamèche, Machoc, Sacha, Charme | Pikachu |
| 7 | on om x | Ronflex, Rondoudou, Bombe, Ombre, Bonbon | Ronflex |
| 8 | an am | Chenipan, Nidoran, Plante, Tranche, Danse | Chenipan |
| 9 | g ge gi | Goupix, Grodoudou, Magicarpe, Charge, Rage | Goupix |
| ★ | *révision de tout* | **Arène de Glace** | **Artikodin** (après la route 3) |
| ★ | *révision de tout* | **Arène de l'Orage** | **Électhor** (après la route 6) |
| ★ | *révision de tout* | **Arène Psy** | **Mewtwo** (après la route 9) |

## Suite à écrire (semaines 3–4)

| Route | Sons | Cibles Pokémon |
|---|---|---|
| 10 | en | Tentacool |
| 11 | eu au eau | **Dracaufeu**, type Feu |
| 12 | z rr nn | **Bulbizarre**, attaque Tonnerre |
| 13 | oi | Poissirène, Boîte |
| 14 | qu | Aquali, Vive-Attaque |
| 15 | in ain | Insécateur, Poing |
| 16 | ill gn | Papilusion, Torgnole |
| 17 | ph y | Métamorph, Ponyta |
| 18 | ce ci | **Carapuce**, Insécateur |

Les graphèmes sont déjà tous définis dans `data/graphemes.json` : il n'y a que
des fichiers d'épisode à écrire, et `pokecontent check` refusera tout mot qu'il
ne peut pas encore lire.

---

## Déploiement

En local, sur le réseau de la maison : `make run`, et c'est tout.

En ligne, à côté d'un site existant, sous un sous-chemin :

```bash
make content                        # la voix (macOS uniquement) doit exister avant le build
docker compose up -d                # standalone, http://<host>:8080/
```

Sous `https://delvecch.io/pokelecture/` : voir **[deploy/delvecch.io.md](deploy/delvecch.io.md)**.

### Le prénom de l'enfant

`POKELECTURE_NAME` (ou `-name`, ou `make run NAME=…`) est lu **à chaque
démarrage** et écrase le prénom stocké dans la sauvegarde : renommer ne coûte
qu'un redémarrage et ne touche ni la progression, ni le Pokédex, ni les étoiles.
Laissé vide, la sauvegarde garde le prénom qu'elle a déjà — `Dresseur` sur une
sauvegarde neuve.

### Comment le sous-chemin fonctionne

`POKELECTURE_BASE=/pokelecture/` fait deux choses : le serveur réécrit
`<base href>` dans les pages, et retire le préfixe avant son propre routeur.
Comme **toutes** les URLs émises sont relatives (`audio/…`, `sprites/…`,
`api/…`), elles se résolvent contre ce `<base href>`.

Résultat : le proxy n'a rien à réécrire, et le **même** binaire tourne à la
racine en local et sous un sous-chemin en production. C'est la partie qui casse
d'habitude quand on monte une app sous un path.

### Ce qui n'est pas dans l'image

- **La voix** est produite par `say` (macOS) : elle voyage dans le contexte de
  build. Comme le serveur construit l'image depuis un **clone du dépôt**, elle
  est committée — sinon la production part sans aucune voix, et sur une tablette
  Android le filet de sécurité TTS ne rattrape rien (pas de voix française
  installée, `getVoices()` vide). Après tout `make content` : commite
  `web/audio/`, sinon la tablette ne l'entendra jamais.
- **Les sprites** sont téléchargés au build depuis PokeAPI — jamais commités.
- La sauvegarde (`var/`) et tes enregistrements (`web/audio/recorded/`) sont des
  volumes : ne les perds pas.

⚠️ Le studio voix et la progression n'ont **aucune authentification**. Sur une URL
publique, mets un `auth_basic` — c'est expliqué dans le guide de déploiement.

---

## Note sur les assets Pokémon

Les sprites viennent de [PokeAPI/sprites](https://github.com/PokeAPI/sprites) et
les noms, personnages et attaques sont la propriété de Nintendo / Game Freak /
The Pokémon Company. C'est un projet familial qui tourne sur ton réseau local —
garde-le comme ça, ne le publie pas. Les textes du Pokédex sont réécrits ici en
français simple, ils ne sont pas copiés des jeux.
