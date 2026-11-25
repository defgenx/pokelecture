# Pokélecture

Apprendre à lire en français (méthode syllabique) avec les Pokémon. Serveur Go +
frontend statique, tout tourne en local sur le réseau de la maison, accessible
depuis une tablette.

---

## Démarrage

```bash
make content            # valide le contenu, génère la voix (ElevenLabs), télécharge les sprites
make run NAME=Prénom    # affiche les URLs du réseau local
```

Puis sur la tablette : ouvrir `http://192.168.x.x:8080` (l'adresse est affichée
au démarrage) → **Ajouter à l'écran d'accueil** (menu Chrome sur Android ;
bouton Partager → « Sur l'écran d'accueil » dans Safari sur iPad).

### Tablettes : iPad et Pixel Tablet

Le jeu est conçu pour l'une comme pour l'autre : icône d'application générée
(fond jaune, y compris la variante *maskable* pour les launchers Android),
portrait **et** paysage, zones sûres (barre d'état, encoche), pincer-zoomer et
appui long neutralisés (des doigts de cinq ans touchent partout), et l'écran
reste allumé pendant une séance (wake lock, en HTTPS).

**Plein écran** : installé sur l'écran d'accueil, le jeu est d'office en plein
écran (manifeste `"display": "fullscreen"`). Dans un onglet de navigateur, il
passe en plein écran au toucher de COMMENCER, et un bouton dans la barre du
haut (ou la touche `F`) l'enlève ou le remet ; le choix est mémorisé sur
l'appareil. Android (et iPadOS) quittent le plein écran dès que le jeu passe en
arrière-plan (autre appli, écran verrouillé) : au retour, le premier toucher le
rétablit, sauf s'il a été coupé avec le bouton. Safari sur iPhone ne permet
pas le plein écran d'une page : le bouton n'y apparaît pas, l'installation sur
l'écran d'accueil le remplace. Sur Android, en HTTP simple, « Ajouter à l'écran
d'accueil » crée un raccourci qui s'ouvre dans un onglet : c'est alors le plein
écran automatique qui joue ; le vrai mode application installée demande HTTPS.

Sur un **téléphone** (largeur ≤ 600 px), la barre du haut de la carte se
resserre : plus de lentille ni de voyants décoratifs, des pastilles et des
boutons plus petits (34 px au moins sous le doigt), les objets du sac (Super
Balls, baies, série de jours) masqués — ils se montrent là où ils servent — et
le bouton 📘 aussi, le Pokédex étant un lieu de la carte. Les boutons ne
rétrécissent jamais ; si un écran très étroit manque encore de place, c'est la
bande des scores qui défile. Tout tient sans défilement à partir de 360 px.

En **paysage**, la mise en page change vraiment : arène de combat à gauche et
mots à droite (plus de défilement en plein combat), carte des routes sur deux
colonnes, actions et vitrine à badges côte à côte, et les très grands corps de
texte se resserrent sur les écrans courts.

Côté fluidité : l'audio de chaque écran est **préchargé** avant le premier
geste, et les garde-fous audio sont proportionnés à la ligne — l'ancien
garde-fou fixe de 9 secondes sur la voix de synthèse était lui-même le blocage
qu'il était censé éviter (une tablette sans voix française gelait le jeu à
chaque ligne manquante).

Une seule asymétrie à connaître : l'iPad ne lit pas les enregistrements WebM.
Le studio voix enregistre désormais en **AAC (.m4a)** quand le navigateur le
permet (Chrome récent, Safari) — ces enregistrements passent partout. Un vieil
enregistrement `.webm` continue de marcher sur Android mais retombe sur la voix
de synthèse côté iPad : ré-enregistre la ligne et c'est réglé.

**Plusieurs enfants** : une sauvegarde par joueur, `var/players/<id>.json`. Au
premier lancement, le jeu demande **« Comment tu t'appelles ? »** (un grand
tape le prénom) ; un autre enfant s'ajoute par **➕ Nouveau joueur** sur l'écran
« Qui joue ? » (menu ☰ → 👥), ou dans l'administration. Depuis le jeu, on ne
peut que nommer un joueur qui n'a pas encore de prénom (renommer reste dans
l'admin), et 12 joueurs au plus ; dans le jeu, l'écran **« Qui joue ? »** montre le Pokémon partenaire de
chacun (il ne sait pas lire les prénoms, il reconnaît son Pokémon), et chaque
appareil retient son joueur (cookie, un an) — **👥 Changer de joueur** dans le
menu ☰. Tout est par joueur (Pokédex, partenaire, badges, missions, records,
réglages de déblocage et de vies) ; seuls la voix et les enregistrements du
studio sont communs. Supprimer un joueur déplace sa sauvegarde dans
`var/players/trash/` (récupérable). Une sauvegarde d'avant les joueurs
(`var/progress.json`) devient le premier joueur au démarrage, l'original étant
gardé en `progress.json.before-players` ; `POKELECTURE_NAME` ne s'applique que
tant qu'il n'y a qu'un joueur.

**🔒 Privé / 🌐 Public** : dans l'administration (onglet Réglages), un joueur
peut être rendu privé par un **code de quatre Pokémon, dans l'ordre**, choisis
parmi neuf (Pikachu, Salamèche, Carapuce, Bulbizarre, Évoli, Rondoudou,
Miaouss, Ronflex, Psykokwak) — un enfant qui ne lit pas peut le taper. Sur
« Qui joue ? », un joueur privé porte un 🔒 ; le toucher ouvre l'écran du code
(dit à voix haute), demandé **une fois par appareil**. Un joueur public
s'ouvre d'un toucher, comme avant ; « Rendre public » retire le code.

- Le code n'est stocké que haché et salé (`lock` dans la sauvegarde) : il ne
  peut pas être réaffiché — notez-le. Ni la liste des joueurs, ni l'état, ni
  l'admin ne l'exposent.
- Le cookie de l'appareil est signé (HMAC, clé `var/players/.device-key`,
  créée au premier démarrage) et lié au code : un cookie fabriqué à la main
  n'ouvre pas une partie privée, et **changer le code le redemande sur tous
  les appareils**.
- Codes faux, comptés dans la sauvegarde (un redémarrage ne les efface pas) :
  après 5, cinq minutes d'attente ; après 10, une heure ; à **15, le joueur
  est fermé** jusqu'à ce qu'un parent clique **🔓 Rouvrir** dans l'admin (le
  code et les appareils restent). Quelqu'un qui essaie depuis Internet a donc
  au plus 15 chances sur 6561 (0,2 %). Les appareils déjà entrés continuent de
  jouer : les essais d'un autre ne bloquent jamais la tablette de l'enfant.
- Chaque essai est vérifié et compté d'un seul coup : une rafale d'essais
  simultanés n'en teste pas plus.
- La liste publique « Qui joue ? » ne montre d'un joueur privé que son prénom
  et son Pokémon. Les sauvegardes sont lisibles par leur seul propriétaire
  (`var/players/` en 0700, fichiers en 0600).

La sauvegarde de chaque joueur est un simple fichier JSON. `make reset`
repart de zéro — ou, plus fin, l'**administration** sur l'ordinateur :
`http://localhost:8080/admin` (voir plus bas).

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
cmd/pokecontent      CLI de contenu : check | audio | sprites | prune
internal/curriculum  chargement + VALIDATION du contenu, génération des séances
internal/progress    sauvegarde JSON + répétition espacée (5 boîtes)
internal/speech      texte → clé de fichier audio stable
data/                le contenu (c'est ici que tu passeras ton temps)
web/                 frontend : HTML/CSS/JS, aucun build
web/audio/           .mp3 générés (ElevenLabs, Jeanne) — **versionnés** (voir Déploiement)
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

### Arènes légendaires, badges et Ligue

Un épisode avec `"legendary": true` ne déclare **aucun** contenu : il tire ses
syllabes, ses mots et ses phrases de **tout ce qui a été appris avant**. C'est
donc à la fois une vraie révision cumulée et le moment où un Pokémon légendaire
entre au Pokédex. Le combat est plus long (5–6 PV, 6 mots) et l'écran passe en
violet et or.

Une arène porte en plus un `"badge": {"id": "glace", "name": "Badge Glace"}` :
le badge est remis en cérémonie après la victoire, et la collection s'affiche
sur l'écran d'accueil (gris tant qu'il n'est pas gagné). Les id valides sont
listés dans `curriculum.Badges` (Go) et dessinés dans `web/icons.js` (`BADGES`) —
une faute de frappe fait échouer `pokecontent check`.

Livrées : **Artikodin** (Badge Glace) après la route 3, **Électhor** (Badge
Orage) après la route 6, **Mewtwo** (Badge Psy) après la route 9, **Sulfura**
(Badge Volcan) après la route 12, **Lugia** (Badge Marée) après la route 15,
**Ho-Oh** (Badge Arc-en-ciel) après la route 18.

Après la route 18 : **le Conseil des 4** — quatre combats légendaires
consécutifs (Suicune, Entei, Celebi, Zoroark), sans badge, comme dans les vrais
jeux — puis **le Champion** : Mew, le Pokémon le plus rare, et le **Trophée du
Champion**.

### Le Pokédex se complète en lisant

Terminer un épisode attrape **tous** les Pokémon rencontrés dedans : la
récompense, le boss, et chaque Pokémon lu dans un mot ou une phrase. Lire un
nom, c'est l'attraper. `pokecontent check` échoue si un Pokémon de
`pokemon.json` n'est attrapable dans aucun épisode : une silhouette qu'on ne
peut jamais remplir est exactement le genre d'impasse que ce jeu s'interdit.

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

## La voix

**Une seule voix, pré-enregistrée : Jeanne (ElevenLabs), en MP3.** Chaque
ligne — sons, syllabes, mots, phrases, consignes — est générée une fois,
enregistrée dans `web/audio/<clé>.mp3` et versionnée. Le jeu ne synthétise
jamais rien en direct : un clip manquant reste muet (avec un avertissement
`[voix]` dans la console), la voix du navigateur n'est jamais utilisée — elle
lisait les syllabes françaises dans une autre voix, souvent anglaise.

La clé et la voix sont dans `.env.local` (ignoré par git, posé par
`npx -y @elevenlabs/cli@latest onboard init`) :

```bash
ELEVENLABS_API_KEY=…
ELEVENLABS_VOICE_ID=txtf1EDouKke753vN8SL   # Jeanne, choisie à l'oreille
```

```bash
make voice-plan                               # ce qui reste à faire et son coût — n'envoie rien
make audio                                    # génère ce qui manque (dans la limite des crédits)
scripts/voice-elevenlabs.sh --voices          # les voix du compte
scripts/voice-elevenlabs.sh --try "Pikachu"   # écouter une ligne (coûte ses caractères)
```

**Chaque ligne n'est payée qu'une fois.** `web/audio/voices.json` note quelle
voix, quel modèle et quelle révision de réglages ont fait chaque clip ; une
relance saute tout ce qui est déjà fait, sur n'importe quelle machine. Le
script s'arrête au budget du mois (sons, syllabes, mots d'abord, phrases
ensuite) et, sur un quota épuisé, s'arrête proprement en gardant ce qui est
fait ; relancer reprend là où il s'est arrêté.

Le MP3 est gardé tel que le service le rend — seul le silence de fin (~0,35 s,
entendu comme un temps mort entre deux syllabes) est coupé, **sans
réencodage** : on retire les trames MP3 muettes (`cmd/pokecontent/mp3trim.go`).
`afconvert` (macOS) sert à décoder pour trouver où la voix s'arrête.

### Le français sur une syllabe nue

Seule, une syllabe ne dit pas au modèle dans quelle langue elle est : « o »
sortait en « oh » anglais, même avec `language_code: fr`. Réglage choisi à
l'oreille parmi cinq variantes (`cmd/pokecontent/elevenlabs.go`), pour les
sons et les syllabes :

- un **contexte français non prononcé** (`previous_text` : « Écoute bien ce
  son, en français : ») ;
- un **accent circonflexe** sur la voyelle seule d'une syllabe ouverte, pour la
  voix seulement : « do » → « dô », « pi » → « pî », « lu » → « lû » (même son,
  graphie uniquement française) ; les digraphes (« ou », « eau ») et les
  nasales ne sont pas touchés ;
- la **normalisation coupée** : c'est elle qui transformait « dre » en
  *Docteure* et « min » en *minute*.

Le modèle `eleven_multilingual_v2` a été écarté (rendu cassé à l'écoute).
Changer un réglage ? Incrémente `elevenRev` : la prochaine génération refait
(et repaie) les clips concernés.

### Contrôler la prononciation : `pokecontent audit`

Une voix de synthèse lit mal un morceau nu de trois façons, toutes
silencieuses : elle l'**épelle** (« pho » → P-H-O, « chu » → C-H-U), le lit
comme une **abréviation** (« dre » → *Docteure*, « min » → *minute*) ou comme
un **mot étranger** (« bin », « man »), ou dit le **nom** de la lettre (« be »
→ *bé*). `pokecontent audit` liste les clips sons/syllabes du plus long au plus
court : un clip épelé dure deux fois plus qu'un clip prononcé, c'est là qu'il
faut écouter.

La méthode qui a produit les réécritures actuelles : pour chaque clip suspect,
générer plusieurs graphies candidates, mesurer leur durée et les transcrire
(reconnaissance vocale), garder celle qui n'est ni épelée ni lue comme un sigle.
Deux règles en sont sorties :

- **Consonnes : « eu » plutôt que « e »** quand la syllabe n'est pas un vrai
  mot (`beu`, `peu`, `feu`, `reu`, `gneu`, `kseu`, `bleu`, `dreu`…). « me »,
  « le », « de », « je » sont des mots et restent tels quels.
- **Nasales : un vrai mot homophone** (`sen` → « sang », `ten` → « tant »,
  `pan` → « paon », `min` → « main », `in` → « hein »). Sans ça, la route 10 —
  qui enseigne `en` = /ɑ̃/ — faisait entendre /sɛn/, /tɛn/.

Et `eu` seul se lit /y/ (« j'ai eu ») : le son se dit « eux ».

### Tout ce qui est écrit est dit

Il ne sait pas lire : chaque consigne affichée est lue à voix haute à
l'ouverture de l'écran, et se réécoute en la touchant (🔊). Côté code, c'est
`hint()` dans `web/app.js` ; un mot ou une syllabe joué pendant la consigne
attend qu'elle finisse au lieu de la couper. Une phrase passée à `hint()` ou
`say()` doit figurer dans `UIPhrases` (`internal/curriculum/session.go`), sinon
elle n'a pas de voix : `go test ./...` échoue
(`speech_coverage_test.go`) — et vérifie aussi les consignes des séances.

## Studio voix — enregistrer ta propre voix

Aucun TTS ne prononce correctement « Roucool », « Miaouss » ou « Salamèche », et
les sons de consonnes isolés sont son point faible absolu. La voix de synthèse
n'est donc qu'un **filet de sécurité** : la vraie voix du jeu, c'est la tienne.

```bash
make run
# puis, sur l'ORDINATEUR : http://localhost:8080/admin/studio
```

Le studio est **une page de l'espace parent** (🎙 Studio voix, groupe Voix ;
`/admin/studio` l'ouvre directement, même connexion Google — `web/admin-studio.js`).
Les enregistrements valent pour tout le jeu, pas pour un joueur.

Maintiens **● Enregistrer** pendant que tu parles (une jauge montre le niveau
du micro), relâche : c'est envoyé, puis ta prise est rejouée. Le fichier est
servi à la place de la synthèse **à la même URL**, donc le jeu n'a rien à
savoir. **▶** écoute ce que le jeu dit, **↺** revient à la synthèse.

- **Une tuile par groupe** (sons, syllabes, mots, phrases) : combien sont déjà
  dans ta voix.
- **Filtres** : le groupe, Toutes / À enregistrer / Ma voix, et une recherche ;
  100 lignes à la fois (« Afficher plus »). Les phrases du jeu, dites comme des
  mots, sont rangées dans Phrases.
- **▶ Enregistrer à la suite** : une phrase à la fois, en grand — la synthèse
  d'abord, maintenir (ou la barre d'espace) pour parler, la prise rejouée, puis
  la suivante non enregistrée du groupe ; « Passer », Échap pour arrêter.
- **🎤 Tester le micro** : la jauge quelques secondes, sans rien enregistrer.

Les lignes sont classées **dans l'ordre de la progression** : les sons
d'abord (route 1 en tête), puis syllabes, mots, phrases. Enregistrer juste les
sons, c'est ~5 minutes et c'est là que se joue l'essentiel.

> Le micro exige un contexte sécurisé : ouvre la page sur `localhost`, pas via
> l'adresse IP du réseau. Les enregistrements vont dans `web/audio/recorded/` et
> `pokecontent prune` n'y touche jamais.

## Administration — vérifier, débloquer, réinitialiser

```bash
make run
# puis, sur l'ordinateur : http://localhost:8080/admin
```

**En local, pas de connexion Google** : `make run` compile en **mode dev**
(`go run -tags dev`) et passe `-local-admin`. Au démarrage, il affiche un lien
`http://localhost:8080/admin?k=…` : l'ouvrir une fois sur cet ordinateur
pose un cookie (30 jours, la clé ne change pas d'un redémarrage à l'autre),
puis `/admin` et le studio voix s'ouvrent sans Google. Il faut à la fois la
clé **et** une connexion directe en boucle locale (127.0.0.1 / ::1), adressée
à `localhost`, sans aucun en-tête de proxy (`X-Forwarded-*`, `X-Real-IP`,
`Forwarded`, `Via`, `CF-Connecting-IP`…) : la clé arrête un tunnel ou une
redirection de port (`ssh -R`, socat) qui ferait entrer Internet par la boucle
locale. La
tablette, sur le réseau, reste refusée ; une page d'un autre site ne peut ni
écrire (contrôle d'origine) ni se faire passer pour localhost (contrôle du
`Host`). C'est un **drapeau** et non une variable d'environnement, pour que le
`env_file` d'un déploiement ne puisse jamais l'activer — et le serveur refuse
de démarrer avec `-local-admin` si `POKELECTURE_URL` pointe ailleurs que sur
localhost, ou dans un conteneur. **Le contournement n'existe que dans une build dev** : le code est
derrière le build tag `dev` (`internal/api/localadmin.go`) ; la build normale
— celle de l'image Docker, `make build` — ne le contient pas et refuse
l'option (`flag provided but not defined: -local-admin`). En production,
Google reste obligatoire. `make test` teste les deux builds.

**L'espace parent** : une barre latérale (le joueur affiché, puis les pages
groupées **Suivi** / **Collection** / **Réglages** ; la page ouverte est
retenue sur l'appareil ; sur un téléphone, la barre passe en haut et les
pages défilent de côté) et, en haut de chaque page, **📥 Exporter** (tout ce
que la page sait de ce joueur, en JSON). Les pages :

- **📊 Tableau de bord** — sept tuiles (aujourd'hui, la semaine et son écart
  avec la précédente, la réussite, le temps de réponse, la série, le parcours,
  le Pokédex), le temps de jeu et la réussite des 14 derniers jours, « Ce qui
  coince » (les éléments les plus fragiles et les plus lents, à faire relire),
  la progression région par région, le calendrier d'activité des 60 jours, la
  limite du jour si elle est active.
- **📈 Statistiques** — sur 7, 14, 30 ou 60 jours : temps de jeu, réponses,
  réussite, temps de réponse jour par jour ; par type de question (sons,
  syllabes, mots, phrases) ; les plus lents à lire ; la lecture à voix haute
  test après test ; les examens ; le calendrier.
- **🗺️ Parcours**, **🧠 Révisions**, **🎓 Examens**, **📖 Lecture à voix
  haute**, **📘 Pokédex** (une grille d'images, filtres Tous / Attrapés /
  Manquants, recherche).
- **⚙️ Règles du jeu**, **🎮 Mini-jeux et Tour**, **👤 Joueur** — les
  réglages, chacun sur sa page.

### Statistiques

Chaque réponse est comptée par jour et par type (`State.Days`, 60 jours
gardés, `LogAnswer`), avec son temps quand c'est une bonne réponse
chronométrée (entre 0,3 et 60 s — un tap perdu ou une tablette posée ne
comptent pas) ; chaque élément garde son temps habituel (`Item.MS`, une
moyenne glissante). Chaque séance et chaque mini-jeu ajoute sa durée (une
heure au plus) — `LogSession`. L'admin reçoit ces jours déjà alignés
(`adminStats`) et les dessine en SVG à la main (`web/admin-stats.js`) : une
série par graphique, une seule couleur, une info-bulle sur chaque barre, point
ou case, et « Voir les chiffres » pour le tableau. L'historique commence le
jour de cette version : avant, rien n'était enregistré jour par jour.

Les pages ci-dessous agissent immédiatement sur la sauvegarde du joueur affiché :

- **Épisodes** : parties jouées, étoiles, dernière session. « ↺ Rejouer comme
  neuf » efface les stats d'un épisode (les étoiles déjà gagnées restent
  acquises) ; « ✓ Marquer terminé » débloque la suite et remplit le Pokédex
  comme une vraie victoire — le bouton pour sauter un épisode qui bloque.
- **Révisions** : chaque syllabe/mot/phrase avec sa boîte de répétition espacée,
  du plus fragile au plus solide — c'est la liste de ce qui coince vraiment.
- **Pokédex** : attraper / retirer n'importe quel Pokémon à la main.
- **Réglages** : renommer l'enfant sans redémarrer ; **la politique de
  déblocage** — terminer suffit (défaut), ou exiger de ★ à ★★★★★ avant de
  débloquer la suite, ou tout débloquer (**mode libre** : toutes les routes et
  régions, mais aussi la Tour, les Dresseurs, les hautes herbes, la canne à
  pêche et les mini-jeux, comme si tout le cours était fait —
  `curriculum.LastDone`) — avec, dans l'onglet
  Épisodes, **un seuil propre à chaque épisode** (« ★ pour la suite ») qui
  prime sur la règle globale ; l'étage de la Tour et les records des
  mini-jeux ; et la remise à zéro complète (le prénom et ces réglages sont
  conservés). « Rejouer comme neuf » retire du total les étoiles de l'épisode
  remis à zéro **et ses Pokémon quittent le Pokédex** (sauf si un autre
  épisode terminé ou la Tour les justifie) — tout se regagne en rejouant,
  jamais en double. Les **vies en combat** se règlent ici aussi.

### Les missions du jour

Chaque jour, des missions 📜 (bouton sur la carte, lues à voix haute), chacune
payée d'une récompense (Poké Balls, baie, Super Ball). Quinze sortes
(`questDefs` dans `internal/api/quests.go`, textes dans `curriculum.QuestPhrases`) :
attraper deux Pokémon dans les herbes, lire dix mots sans faute, terminer une
route, battre un Dresseur, trouver un objet caché, donner une baie, jouer à un
mini-jeu, **monter un étage de la Tour, pêcher un Pokémon, lire son livre
jusqu'au bout, battre un record, atteindre le niveau 3 dans un mini-jeu,
téracristalliser son partenaire, gagner cinq étoiles sur une route, gagner un
combat sans une seule erreur**. Le tirage du jour (`questsFor`) ne garde que
**les missions possibles pour cette sauvegarde** (`questPossible`) : pas de
pêche sans la canne, pas de livre avant « Mon livre », pas de Tour ni de
Dresseur avant la première route — sinon une mission impossible tiendrait la
journée ouverte sous la limite. Sans limite : les trois premières du tirage.

### La limite du jour

Réglage **« Limiter la journée »** (oui / non) — sans lui, il joue autant qu'il
veut — puis deux nombres : **les objectifs du jour** (1 à 7, 3 par défaut :
`DailyMissions`) et, **dont, les leçons** (1 à 5, 2 par défaut). L'admin le dit en
une phrase : « Chaque jour, 3 objectifs : 2 leçons + 2 missions au hasard. ». Une
leçon = une route terminée, rejouée ou non, ou un examen (`Today.Lessons`, remis
à zéro chaque jour).

**Les objectifs sont les missions du jour** (📜) : la première est toujours les
leçons (« Fais deux leçons aujourd'hui. », payée comme les autres), les autres
sont tirées au hasard, le même tirage toute la journée (`questsFor` mélange
toutes les missions, `State.ActiveQuests` en prend le début) ; sous la limite,
les missions qui demandent une route (« Termine une route », « Gagne cinq
étoiles… » : `RouteQuests`) ne sont jamais tirées — les routes ferment une fois
les leçons faites, et les leçons les couvrent. 1 objectif =
les leçons seules. Sans la limite : les trois missions habituelles. Deux étapes :

1. **Leçons faites** (`LessonsOver`) : plus de routes ni d'examens — un écran 📚
   « Tes leçons du jour sont faites ! Termine tes missions pour finir la
   journée. » avec le bouton des missions. La Tour, les Dresseurs, les
   mini-jeux, les herbes et la pêche restent ouverts pour finir les missions.
2. **Toutes les missions faites** (`DayOver`) : plus rien jusqu'au lendemain —
   écran 🌙 et la voix « Bravo, toutes tes missions du jour sont faites !
   Reviens demain. », dite une fois par la mission qui ferme la journée.

Sur la carte, la puce 📅 1/2 devient 📅 ✓ puis 🌙. La séance en cours se
termine normalement ; c'est la suivante qui ne démarre pas. Le serveur refuse
aussi (403) : routes et examens dès les leçons faites ; tout (séances, tickets,
captures) une fois la journée finie. L'onglet Réglages montre les leçons faites
aujourd'hui. Une mission impossible ce jour-là (plus de Poké Balls pour
« Attrape deux Pokémon ») garde la journée ouverte : les routes, elles, restent
fermées. Indépendant du mode libre.

### Se connecter avec Google

L'administration et le studio voix ne s'ouvrent **qu'aux adresses de
`POKELECTURE_ADMINS`**, connectées avec Google — pages **et** endpoints
(`api/admin/*`, `api/texts`, `api/record/*`). Sans configuration, ils sont
**fermés à tout le monde** (pas de liste, pas d'accès). Le jeu de l'enfant,
lui, n'est jamais derrière.

1. [Google Cloud Console](https://console.cloud.google.com/apis/credentials) →
   *Identifiants* → *Créer des identifiants* → *ID client OAuth* → type
   *Application Web*.
2. *URI de redirection autorisés* — exactement, caractère pour caractère :
   `http://localhost:8080/auth/callback` (en local) et
   `https://delvecch.io/pokelecture/auth/callback` (en ligne). Google refuse
   le `http://` sur une adresse IP du réseau (`192.168.…`) : l'admin se
   consulte alors sur `localhost` ou en HTTPS.
3. *Écran de consentement OAuth* en mode *Test* suffit : ajoute les adresses
   autorisées comme *utilisateurs test*.
4. Dans `.env.local` (ignoré par git ; `make run` le lit) ou les variables du
   conteneur :

```bash
GOOGLE_CLIENT_ID=…apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=…
POKELECTURE_ADMINS=toi@gmail.com,autre@gmail.com   # les seuls comptes acceptés
POKELECTURE_SESSION_KEY=…   # openssl rand -hex 32 — 32 caractères minimum, sinon le serveur refuse de démarrer
POKELECTURE_URL=https://delvecch.io/pokelecture/   # en ligne : toujours le mettre
```

Règles de sécurité (relecture indépendante du code faite) :

- **Le compte Google ne risque rien** : Pokélecture ne voit jamais le mot de
  passe, ne demande que `openid email`, pas d'accès hors ligne, aucun jeton
  gardé. Le pire d'un bug serait l'accès à l'admin Pokélecture.
- **`POKELECTURE_SESSION_KEY` longue et aléatoire** : une clé courte se casse
  hors ligne et permettrait de forger une session. Changer la clé déconnecte
  tout le monde.
- **En ligne, toujours `POKELECTURE_URL`** : alors aucun en-tête
  `X-Forwarded-*` envoyé par un client n'est cru.
- **De préférence des adresses Gmail** dans la liste : pour une adresse sur un
  domaine à soi, c'est celui qui contrôle le domaine qui contrôle l'identité.
- **Un secret qui fuit** (`GOOGLE_CLIENT_SECRET`) : le régénérer dans la
  console Google. Les secrets vivent dans `.env.local` ou `.env`, ignorés par
  git et par Docker.
- Les enregistrements du studio voix doivent être de l'audio (MP4/M4A ou WebM,
  vérifié aux premiers octets), ne sont servis qu'avec un type audio, et aucune
  réponse ne laisse le navigateur deviner un type (`nosniff`).

Les adresses : **`/admin`** (l'administration) et **`/admin/studio`** (le
studio voix) ; les anciennes `admin.html`, `parent.html` et `/studio`
redirigent vers elles. On y
arrive depuis le jeu par le lien **🔧 Espace parents**, en bas du menu ☰.
Sans session, la page affiche un bouton **« Se connecter avec Google »** — elle
n'envoie jamais d'elle-même vers Google ; déjà connecté, on arrive directement
sur l'admin, et « Se déconnecter » ramène au bouton.

La session dure 30 jours (cookie signé, `HttpOnly`) ; retirer une adresse de
`POKELECTURE_ADMINS` la révoque au redémarrage suivant. Le jeton d'identité est
reçu directement de Google en HTTPS, ses champs (client, expiration, nonce,
adresse vérifiée) sont contrôlés — `internal/api/auth.go`, testé contre un faux
Google dans `auth_test.go`.

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
| Jamais de voix de synthèse en direct | la voix du navigateur lisait les syllabes françaises dans une autre voix (souvent anglaise) ; un clip manquant reste muet et `go test` le signale |
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

Le combat est mis en scène comme dans les jeux : écran **VS** où les deux
sprites déboulent face à face, mot reconstruit = attaque, **COUP CRITIQUE !**
quand le mot est bâti sans une seule erreur, riposte du boss (charge + bruit
sourd, jamais de dégâts pour l'enfant — perdre n'existe pas à 5 ans), et
**COMBAT PARFAIT !** avec confettis quand tous les mots sont passés du premier
coup.

Le son fait le reste : `web/sfx.js` synthétise tout au Web Audio, sans un seul
fichier — carillon montant sur une bonne réponse, whoosh d'attaque, arpège de
capture, grondement à l'entrée d'un légendaire, fanfare de badge. Plus des
confettis sur une capture, un badge et en fin de séance.

---

## Épisodes livrés (routes 1 à 24, 8 badges, puis la Ligue)

Comme dans les vrais jeux : **les huit badges d'abord, le Conseil des 4
ensuite**, et le Champion en toute fin de parcours.

| Route | Sons | Vocabulaire Pokémon | Récompense |
|---|---|---|---|
| 1 | a i o u é e m l r t s n tt ss | Otaria, Rattata, Ramoloss, Amonita, Morsure, Normal | Rattata |
| 2 | p d | Doduo, Dodrio, Nidorina, Paras, Tornade | Doduo |
| 3 | f v | Mélofée, Évoli, Nosferapti, Fée, Vol, Vitesse | Évoli |
| ★ | *révision de tout* | **Arène de Glace** · Badge Glace | **Artikodin** |
| 4 | b c | Abo, Sabelette, Caninos, Cascade, Roc | Caninos |
| 5 | ou oo | Roucool, Miaouss, Poudre, Roc, Coupe | Roucool |
| 6 | ch è k | **Pikachu**, Salamèche, Machoc, Sacha, Charme | Pikachu |
| ★ | *révision de tout* | **Arène de l'Orage** · Badge Orage | **Électhor** |
| 7 | on om x | Ronflex, Rondoudou, Bombe, Ombre, Bonbon | Ronflex |
| 8 | an am | Chenipan, Nidoran, Plante, Tranche, Danse | Chenipan |
| 9 | g ge gi | Goupix, Grodoudou, Magicarpe, Charge, Rage | Goupix |
| ★ | *révision de tout* | **Arène Psy** · Badge Psy | **Mewtwo** |
| 10 | en | Tentacool, Tentacule, Encore, Tente | Tentacool |
| 11 | eu au eau | **Dracaufeu**, Feu, Eau, Bateau, Cadeau, Chaleur | Dracaufeu |
| 12 | z rr nn | **Bulbizarre**, Tonnerre, Gazon, Zéro | Bulbizarre |
| ★ | *révision de tout* | **Arène du Volcan** · Badge Volcan | **Sulfura** |
| 13 | oi | Poissirène, Mimitoss, Étoile, Miroir, Poire, Soir | Poissirène |
| 14 | qu | Attaque, Vive-Attaque, Casque, Quatre | Aquali |
| 15 | in ain | Insécateur, Insecte, Matin, Lapin, Main, Train | Insécateur |
| ★ | *révision de tout* | **Arène de la Mer** · Badge Marée | **Lugia** |
| 16 | gn ill | Montagne, Papillon, Torgnole, Grognon (boss : Aspicot) | Papilusion |
| 17 | ph y | Métamorph, Photo, Stylo, Trophée | Ponyta |
| 18 | ce ci | **Carapuce**, Racine, Cercle, Crabe, Pouce | Carapuce |
| ★ | *révision de tout* | **Arène Arc-en-ciel** · Badge Arc-en-ciel | **Ho-Oh** |
| 19 | j | Judo, Pyjama, Jeudi, Journal, Joie | **Jirachi** |
| 20 | ai ei ê | Baie, Reine, Pêche, Aile, Neige | **Roucarnage** (Roucool a évolué) |
| 21 | mm pp ll ff | Flammèche, Pomme, Balle, Griffe | **Mackogneur** (Machoc) |
| ★ | *révision de tout* | **Arène du Tonnerre** · Badge Tonnerre | **Raikou** |
| 22 | oin ien | Gardien, Pointu, Lointain, Chien, Soin | **Arcanin** (Caninos) |
| 23 | h ui | Parapluie, Buisson, Ruisseau, Pluie, Huit | **Léviator** (Magicarpe !) |
| 24 | er | Rocher, Panier, Chanter | **Florizarre** (Bulbizarre) |
| ★ | *révision de tout* | **Tour du Ciel** · Badge du Ciel | **Rayquaza** |
| ★ | *Conseil des 4, 1/4* | combat légendaire | **Suicune** |
| ★ | *Conseil des 4, 2/4* | combat légendaire | **Entei** |
| ★ | *Conseil des 4, 3/4* | combat légendaire | **Celebi** |
| ★ | *Conseil des 4, 4/4* | combat légendaire | **Zoroark** |
| 🏆 | *tout* | **Le Champion** · Trophée du Champion | **Mew** |
| 28 | tion | Évolution, Attention, Station, Phyllali, Givrali | **Nymphali** |
| 29 | ail eil aille eille | Soleil, Corail, Bataille, Abeille, Médaille, Apireine, Riolu | **Lucario** |
| 30 | œu oeu ç | Cœur, Sœur, Œuf, Leçon, Garçon, Noeunoeuf, Grelaçon, Tarsal, Kirlia | **Gardevoir** |
| 31 | ette elle esse enne | Floette, Flabébé, Raquette, Antenne, Poubelle, Vitesse, Lougaroc | **Mimiqui** |
| ★ | *révision de tout* | **Arène du Savoir** · Diplôme de l'Académie | **Meloetta** |

Sur la carte, les quatre portes du Conseil sont numérotées **1/4 à 4/4** et
le Champion porte 🏆. On n'en voyait que trois : le partenaire, posé sur la
prochaine porte, était dessiné par-dessus et la cachait. Sur un arrêt, il se
fait maintenant plus petit, perché dans le coin, et l'arrêt reste visible.

Les routes 19–24 sont **les Îles Lointaines**, sur le thème de l'évolution :
les Pokémon attrapés au début de l'aventure ont grandi avec le lecteur.

### Les nouveaux Pokémon dans l'aventure

Les Pokémon de la Tour se montrent bien avant leur étage :

- **Chaque écran d'histoire** a un Pokémon qui jette un œil (👀) : un Pokémon de
  la Tour pas encore attrapé, différent selon les jours, avec la ligne « Il
  t'attend à l'étage… » (touchable, déjà dans la voix).
- **Les histoires de l'Académie les nomment** (Alakazam le directeur,
  Typhlosion, Leveinard, Spectrum, Tyranocif, Méganium) : le champ `cameo` d'un
  épisode affiche leurs images au-dessus du texte ; `pokecontent check` refuse
  un cameo que l'histoire ne nomme pas.
- **Sur la carte**, la 🗼 porte le Pokémon du prochain étage qui en donne un,
  et chaque Dresseur montre (et dit) le Pokémon qu'il envoie aujourd'hui.

### Les régions et la carte du monde

Chaque épisode déclare sa **région** (`"region": "Forêt Émeraude"`) : Forêt
Émeraude, Colline Azur, Grotte Violette, Volcan Rougeoyant, Côte Saphir,
Vallée Arc-en-ciel, Îles du Levant, Îles du Couchant, Plateau de la Ligue —
puis, après le Champion, **Terres Nouvelles** et **Failles Ultra**. La liste
de l'accueil est groupée par région, et le bouton **🗺️ Carte** ouvre la carte
du monde : une carte par région, ses épisodes en sentier de Poké Balls
(colorée = terminé, dorée qui pulse = prochaine étape, grise = verrouillé).

### Après le Champion : Terres Nouvelles et Failles Ultra

Du contenu de **consolidation** (aucun nouveau son : tout l'inventaire CP est
déjà là) organisé **par jeu Pokémon** — les starters de Hoenn/Sinnoh,
Unys/Alola et Galar/Paldea (Poussifeu, Gobou, Tiplouf, Moustillon, Brindibou,
Flamiaou, Flambino, Poussacha, Chochodile…) — puis trois **Failles Ultra**,
des combats légendaires doubles — **quatre**, comme les portes du Conseil :
Dialga **et** Palkia, Lunala **et** Solgaleo, Kyogre **et** Groudon (la
Faille des Abysses), puis Giratina **et** Necrozma, la dernière.

Ajouter un épisode au milieu du parcours ne ferme rien : un épisode déjà
terminé, ou dépassé, reste ouvert ; le nouveau devient simplement la
prochaine étape.

### L'Académie Pokémon : les sons de CE1

Après les Failles Ultra, cinq épisodes « pour grands lecteurs » (routes 28–31
puis l'Arène du Savoir) enseignent ce qui dépasse le CP : `tion` (/sjɔ̃/),
`ail eil aille eille`, `œu`/`oeu` et `ç`, et le **e qui fait è** devant une
consonne double (`ette elle esse enne`). Chaque cours fait lire des noms de
Pokémon d'autres générations (Lucario, Gardevoir, Mimiqui…), attrapés en les
lisant. Pièges écartés : **Nymphali** se dit /nɛ̃fali/ (`ym` n'est pas
enseigné) — c'est la récompense, lue *à* l'enfant, jamais un mot à déchiffrer ;
**Pomdepik** se dit « pomme » (le `om` glouton lirait /pɔ̃/) : il est à la Tour.
« Le son manquant » découpe chaque mot avec les sons connus **sur sa route**
(« Aile » reste a-i-l-e, pas le `ail` de l'Académie).
Ces épisodes forment le quatrième palier de difficulté (6 mots par combat,
3 syllabes intruses).

### Les examens surprises (`data/events.json`)

Un **examen surprise** n'est pas une étape du parcours : il **apparaît au
hasard** sur la carte — un 🎓 avec un « ! », à la même case dans chaque
région, annoncé à voix haute — un jour sur `every` (tiré par un hachage de
l'examen et de la fenêtre de jours : aléatoire, mais stable sur la journée),
**une fois son épisode `after` terminé**, et tant qu'il n'est pas réussi.

| Examen | Apparaît après | Questions (sons / syllabes / mots / phrases) | Réussite | Pokémon |
|---|---|---|---|---|
| Examen surprise | Arène de Glace (1ᵉʳ badge) | 6 / 5 / 5 / 2 | 70 % | Créfadet |
| Examen surprise | Arène Psy (3ᵉ badge) | 7 / 5 / 5 / 3 | 75 % | Créfollet |
| Le Grand Examen | Tour du Ciel (8 badges) | 8 / 4 / 5 / 3 | 80 % | Créhelf |
| L'Examen de Maître | Arène du Savoir | 8 / 4 / 5 / 3 | 85 % | Arceus |

**Un examen jamais tenté attend chaque jour** sur la carte, dès qu'il est
ouvert : le premier doit se voir. Ensuite seulement il devient surprise. Le
menu ☰ a une page **🎓 Examens** (chaque gardien, en silhouette tant qu'il
n'est pas attrapé : plus tard / sur la carte aujourd'hui / un autre jour /
réussi), et un bouton 🎓 apparaît dans la barre du haut de la carte les jours
d'examen.

**Vingt questions au plus, un examen par semaine au plus** : après une
tentative, aucun autre examen n'apparaît pendant six jours (celui du jour
reste là pour réessayer). Un examen de 30 questions sans aucune aide, c'est
trop long à cet âge, et trois examens éligibles en même temps en faisaient
apparaître un un jour sur deux.

C'est une vraie évaluation : **une seule réponse par question**, aucune aide
avant de répondre (pas de syllabe à toucher, pas de karaoké), la bonne
réponse montrée et dite après une erreur. Les questions vont **du plus fragile
au plus solide** (boîtes de répétition). La section « sons » fait entendre un
son et montrer les lettres qui l'écrivent — sans jamais mettre deux homophones
côte à côte (`o`/`au`/`eau`, `ou`/`oo`, `in`/`ain`, `œu`/`eu`… liste `Homophones` dans
`internal/curriculum/event.go`, la seule : « Le son manquant » la reçoit de
`/api/arcade`), et la section « syllabes » n'offre jamais deux syllabes qui
se disent pareil (`san`/`sen`, `ca`/`ka`, `son`/`çon`). Une tentative qui répond à moins de la moitié
des questions est un échec.

- **Réussi** : le Pokémon entre au Pokédex (+5 Poké Balls), l'examen ne
  revient plus.
- **Raté** : l'écran montre les sons à revoir (touchables, ils se font
  entendre) ; chaque réponse fausse est descendue d'une boîte et revient en
  échauffement — les sons aussi : un son raté à un examen est redemandé
  (« entends-le, touche ses lettres ») au début des séances suivantes. L'examen se retente le jour même, ou réapparaîtra plus tard.
- **Admin, onglet Examens** : la condition, le dernier essai section par
  section, ce qui est à revoir, et **« Faire apparaître aujourd'hui »** pour le
  poser sur la carte sans attendre le tirage.

**L'examen d'entrée.** Un épisode peut déclarer `"requires_exam": "<id>"` :
sa porte reste fermée tant que l'examen n'est pas réussi. C'est le cas de
l'Académie (`ep36` → le Grand Examen) : pour être à l'Académie, il faut
prouver qu'on sait lire. Arrivé devant la porte, le partenaire s'y pose
comme sur une route (une 🎓 sur la Poké Ball), JOUER lance l'examen, et
l'examen attend sur la carte **chaque jour** jusqu'à la réussite (ni tirage,
ni pause d'une semaine). Une sauvegarde déjà entrée avant la porte n'est pas
bloquée ; le mode libre l'ignore.

Ajouter un examen = une entrée de `events.json` (`id`, `title`, `story`,
`after`, `every`, `pass`, `pokemon`, `questions`) ; `pokecontent check`
vérifie l'épisode, le Pokémon et les sections.

### Les étoiles : justesse et vitesse

**Cinq étoiles** : à 6 ans, en CP, cinq se comptent sans peine et laissent
voir les progrès d'un rejeu à l'autre. La vitesse décide de la cinquième :

- ★ : l'épisode est fini ;
- ★★ : 60 % de bonnes réponses ;
- ★★★ : 75 % ;
- ★★★★ : 90 % ;
- ★★★★★ : 90 % **et** au moins 60 % des réponses rapides.

Une sauvegarde de l'époque des trois étoiles est convertie une fois, à
l'ouverture, **sans rien perdre** : ★ → ★, ★★ → ★★★, ★★★ → ★★★★★ (et le
total d'étoiles suit) ; les seuils de déblocage de l'admin sont convertis de
même (`StarScale` dans la sauvegarde). Les Poké Balls et l'expérience gagnées
restent celles d'avant : 5★ rapporte ce que rapportaient 3★.

**Rapide, c'est par rapport à lui-même.** Tant que le jeu ne le connaît pas
(moins de 10 réponses justes chronométrées d'un type), une réponse est rapide
sous 5 s pour un son ou une syllabe, 7 s pour un mot. Ensuite, le serveur
garde **son temps habituel** par type (moyenne glissante, `State.Speed` ; un
toucher sous 0,3 s ou une absence de plus d'une minute ne comptent pas) et
« rapide » veut dire un peu plus vite que d'habitude (×1,25, borné entre
1,5 s et deux fois la limite fixe). Une limite fixe pousse à deviner ; battre
son propre rythme fait progresser. Le chrono part quand la consigne parlée et
la question sont finies. L'écran de fin montre la ligne ⚡ ; aucune phrase ne
lui dit « plus vite ». La vitesse ne fait jamais rater : elle ne retire que la
cinquième étoile. Les
phrases (lues à voix haute, à son rythme) et le combat ne sont **pas**
chronométrés pour l'étoile : sinon taper « J'AI LU ! » sans lire serait
« rapide », et une règle « ★★★★★ pour la suite » pourrait bloquer l'aventure sur
la vitesse d'un enfant de 5 ans.

### Ce que les exercices font travailler (et comment ils évitent les pièges)

- **L'échauffement redemande deux sons** à chaque séance — les plus fragiles
  des routes précédentes : on entend le son, on touche ses lettres. Le lien
  lettre → son est le cœur de la lecture ; il n'était travaillé qu'à sa
  découverte.
- **« Lis le mot » propose des images de mots qui se ressemblent** (même
  première syllabe, sinon même première lettre) : la première lettre ne suffit
  plus à deviner, il faut lire le mot jusqu'au bout. Même règle pour l'image de
  « Lis la phrase », « Lis et attrape » et les noms du « Pokémon mystère ».
- **« Quel mot va avec l'image ? »** — un « Lis le mot » sur deux est
  retourné : l'image est donnée (Pikachu), les réponses sont écrites — le vrai
  mot et des copies où une syllabe change pour une syllabe voisine déjà apprise
  (`Pikachu` / `Pitachu` / `Pikacho`, `spellItem`). L'image dit *quel* Pokémon,
  elle ne peut pas donner la réponse : seule la lecture de chaque syllabe la
  trouve. Les copies gardent la première lettre (sinon un coup d'œil suffit),
  ne sont jamais un mot du cours et ne sont jamais dites (une copie peut être un
  vrai mot, « Change » pour « Charge » : la question demande le mot de l'image,
  pas un mot « bien écrit », donc il n'y a toujours qu'une bonne réponse) ; la voix lit le bon mot
  une fois trouvé. Repli sur la question normale quand un mot n'a pas deux
  copies possibles.
- **Les syllabes en couleur** : partout où un mot écrit s'affiche et que ses
  syllabes sont connues — « Lis le mot », ses copies, les examens, le mot du
  combat, « Lis et attrape », l'atelier, les noms du « Pokémon mystère », les
  paires — elles alternent **bleu / rouge**, comme au CP. Exception : le
  tambour, où les couleurs donneraient le nombre de syllabes demandé (il les
  colore après la réponse). Les mots des phrases restent d'un bloc : leurs
  syllabes ne sont pas décrites dans le contenu.
- **Les lettres muettes en gris** : sur les mêmes mots, plus les phrases (« Lis
  la phrase », « Mon livre », révisions, examens) et le son manquant, ce qui
  s'écrit sans se dire est grisé, comme au CP : le **e final** (`Rag`e,
  `Mélofé`e, `Ail`es — mais pas `le`, `de`, `que`, `les`, dont c'est la seule
  voyelle) et le **h** qui ne fait pas `ch`/`ph`/`sh`/`th` (`H`uit,
  `R`h`inocorne`, `Lok`h`lass`). Règle unique : `muteMask` (`web/app.js`).
  Les autres lettres muettes (t, s, d finaux) n'ont pas de règle : les règles
  de contenu les tiennent hors des mots à lire.
- **Une mauvaise image ne fait pas dire le mot** : « Presque ! Relis le mot. »
  (ou « la phrase »). Avant, la voix lisait le mot après l'erreur : toucher une
  image au hasard, puis écouter, répondait sans lire. Les questions d'écoute
  rejouent toujours le son.
- **Aucune question n'offre deux syllabes qui se disent pareil** (`san`/`sen`,
  `ca`/`ka`, `ta`/`tta`) : `sameSound` (`internal/curriculum/sounds.go`)
  compare les sons, pas les lettres, pour l'écoute, les examens et les
  syllabes intruses du combat.
- **La dictée du combat** : le dernier mot d'une arène (le coup Gigamax ou Dynamax) et
  ceux des étages de la Tour (1 dès le 10ᵉ, 2 au 20ᵉ, 3 au 30ᵉ) sont **cachés** — on l'entend, on
  le reconstruit. Écrire n'est plus recopier. Seulement si aucune tuile ne se
  dit comme une autre (« Rattata », `ta`/`tta`, n'est jamais dicté).
- **L'oreille fine** : un tour sur deux, la syllabe est mise à côté de sa
  **paire minimale** (b/d, p/b, f/v, ch/j, t/d, k/g, s/ch, m/n…), les sons que
  l'oreille d'un enfant confond.

### La police de lecture

Admin, onglet Réglages : les mots, syllabes et phrases à lire s'affichent dans
la police du jeu, en **Andika** (SIL, faite pour les lecteurs débutants : `a`
et `g` d'école) ou en **cursive** (Playwrite France Moderne, l'écriture
attachée de l'école). Le reste de l'écran ne change pas. Les polices sont
servies par le jeu (`web/fonts/`, licence OFL, réduites aux lettres latines et
`œ` : 24 Ko et 98 Ko) et gardées hors ligne par le service worker.

### La lecture chronométrée (admin, onglet Lecture)

Le vrai remplaçant de la reconnaissance vocale : un parent s'assoit à côté
de lui, lance la minute, et pour chaque mot (pris dans ce qu'il a appris)
touche **✓ bien lu** ou **✗ raté**. Résultat : **mots bien lus par minute**,
gardés dans un historique (une fois par semaine suffit pour voir la courbe),
et chaque mot va dans les boîtes de révision. L'onglet montre aussi son temps
de réponse habituel dans le jeu.

### Perdre existe : les vies en combat

Chaque syllabe fausse en combat coûte un cœur (❤️ ×3 par défaut). À zéro :
**DÉFAITE** — l'allié tombe, et l'épisode recommence du début (l'étage, pour
la Tour). Les erreurs comptent quand même pour la répétition espacée.
Configurable dans l'admin : illimitées (l'ancien mode sans enjeu) ou 2 à 5
vies.

### Gigamax et Téracristal

**Le dernier point de vie** : le boss grandit pour le dernier mot. S'il a une
**vraie forme Gigamax** (Florizarre, Dracaufeu, Tortank, Papilusion, Pikachu,
Miaouss, Mackogneur, Ectoplasma, Krabboss, Lokhlass, Évoli, Ronflex —
`curriculum.Gigamax`, les formes « -gmax » de PokeAPI que `pokecontent
sprites` télécharge), c'est **son artwork officiel** qui apparaît : « GIGAMAX ! ».
Sinon — les légendaires n'en ont pas — il **Dynamaxe**, l'apparence officielle
du Dynamax : la même image redessinée chargée d'énergie rouge-rose
(`dynamaxArt`, un canvas), géante. Dans les deux cas, les **nuages rouges**
tourbillonnent au-dessus de lui. Ça vaut pour les arènes et la Tour, et pour tout
combat dont le boss a une forme Gigamax (les routes d'Évoli, Pikachu, Ronflex,
Dracaufeu, Mackogneur, Florizarre ; un Dresseur qui l'envoie).

**Le Téracristal, contre des personnes** — les Dresseurs de la carte, le
Conseil des 4 et le Champion (`TeraRegion`) : un 💎 vide à côté du partenaire.
Un **coup critique** (un mot construit sans une seule erreur) le remplit ; le
bouton « 💎 TÉRACRISTAL ! » (ou le joyau lui-même) cristallise le partenaire —
l'apparence officielle, refaite sur son image faute d'artwork officiel
disponible : corps en cristal à facettes teinté de la couleur officielle de son
type Téra (son premier type, `TERA_COLORS` ; `crystalArt`, un canvas qui ne
touche que ses pixels), et **le joyau Téra** au-dessus de sa tête, de la même
couleur, avec le symbole du type —
et **chaque attaque suivante enlève 2 PV** : le combat se gagne plus vite. Au
premier mot, la voix l'explique ; c'est la récompense de la lecture parfaite.

### Les exercices, plus vivants

Les écrans d'entraînement empruntent aux meilleures apps du genre : **pips de
progression** (des Poké Balls qui se remplissent : on voit ce qui reste),
**combo 🔥** de bonnes réponses d'affilée avec mini-feu d'artifice tous les
trois, et le tempo des réponses a été resserré (le mot confirmé remplace le
« Bravo » parlé).

### Difficulté progressive, alignée CP

Les exercices durcissent avec la progression, par tiers du parcours :

| | Choix par question | Pièges dans le combat | Mots par combat |
|---|---|---|---|
| Routes 1–9 | 3 | 1 syllabe intruse | 4 |
| Routes 10–18 | 4 | 1 | 4 |
| Ligue et Îles | 4 | 2 | 5 (6 en arène) |
| Académie (route 28+) | 4 | 3 | 6 |

Le palier suit **la plus haute route atteinte**, pas la position dans la
liste : insérer un épisode ne décale plus la rampe.

Côté programme : une fois les Îles terminées, l'inventaire couvre **tous les
graphèmes du CP** — voyelles, consonnes, digraphes (`ou oi on an en in au eu
ai ei ch gn qu ce ci ge gi er`), trigraphes (`eau ain oin ien ill`), lettres
muettes (`e`, `h`) et doubles consonnes. L'**ordre** s'écarte de la progression
CP classique pour débloquer du vocabulaire Pokémon au plus vite (choix assumé,
voir « Le principe ») ; le **niveau**, lui, reste strictement CP : mots courts
réguliers, phrases sujet-verbe-complément, aucune notion de CE1 (pas de
`s`=/z/ entre voyelles, pas de `ti`=/si/, pas d'exceptions du `ill`).

### Rejouable : chromatiques et Tour de Combat

- **Pokémon chromatiques ✨** (« Les chromatiques ») : rejouer un épisode déjà
  terminé donne 1 chance sur 6 d'attraper la version chromatique de sa
  récompense — une seconde collection, plus lente, par-dessus le Pokédex.
  Compteur ✨ sur l'accueil. Un chromatique s'affiche avec **son artwork
  chromatique officiel** (`sprites/art/shiny/`, que `pokecontent sprites`
  télécharge pour tous les Pokémon du cours et d'`arcade.json`) et un halo
  doré, partout : la rencontre dans les herbes ou à la pêche, la capture, la
  récompense, le gardien de la Tour, les deux Pokédex, le Pokémon doré de la
  chasse (`asShiny` dans `web/app.js`). Si le fichier manque, l'image normale
  revient avec l'ancienne teinte (`shiny-tint`).
- **🗼 Tour de Combat, par étages** : dès le premier épisode terminé. Chaque
  étage est un combat noté — 2 points par mot construit sans une seule erreur,
  1 sinon — et il faut **au moins la moitié des mots du premier coup** pour
  monter. Les combats s'allongent (4 → 8 mots) et se piègent (1 → 3 syllabes
  intruses) avec les étages, et **les mots durcissent** : ils sont rangés par
  difficulté (nombre de syllabes, plus jusqu'à 3 points pour un mot appris tard
  dans le cours) et chaque étage tire dans une **bande** de ce classement qui
  monte jusqu'aux mots les plus durs à l'étage 30 (`TowerClimb`). Dans la
  bande passent d'abord les mots lus il y a le plus longtemps (`last_seen` de
  la répétition espacée) : d'un étage à l'autre, les mots tournent au lieu de
  revenir (avant, un tri par longueur ressortait toujours les mêmes mots de 4
  syllabes). Dans l'étage, du plus simple au plus dur. Les mots cachés
  (dictée) passent de 1 à l'étage 10 à 2 au 20ᵉ et 3 au 30ᵉ. Et **un étage sur
  deux libère un Pokémon exclusif à la Tour** : vingt Pokémon (Psykokwak,
  Onix, Ectoplasma, Lokhlass… jusqu'à Dracolosse à l'étage 40) qu'on ne peut
  obtenir nulle part ailleurs. **Au-delà de l'étage 40, chaque étage** en
  libère un : 75 de plus, jusqu'à l'étage 115 (Pichu, Nidoking à l'étage 50,
  Steelix au 60, Cizayox au 70, Alakazam au 80, Typhlosion au 90, Tyranocif
  au 100, Méganium au 110…) — tous les Pokémon des générations 1 et 2 qui ne
  vivent pas dans les hautes herbes. Un Pokémon déjà obtenu dans le Pokédex
  sauvage par évolution (Roucoups, Raichu…) compte comme attrapé. Leur ordre est celui des `"tower": true`
  dans `pokemon.json` ; leur ligne « où le trouver » est commune
  (« tout en haut de la Tour »), une ligne de voix par étage coûterait cher. On ne passe pas à l'étage suivant tant que
  l'étage courant n'est pas réussi ; chaque mot lu alimente la répétition
  espacée.
- **Rotation des rediffusions** : quand tout est terminé, JOUER relance
  l'épisode le moins récemment joué.

Pièges de la progression, gérés dans le contenu : « Poing » et « Boîte »
(lettre finale muette / `oî`), « Chenille » (le `en` glouton après la route 10),
« Aquali » et « Ponyta » comme *mots à lire* (`qu` = /kw/, `on` glouton) — ils
n'apparaissent que comme récompenses, dont le nom est lu **à** l'enfant.
« Tonnerre » est découpé `To-nnerre` pour que le `o` ne colle pas au `n`.

Quand tout est terminé, « JOUER » relance l'épisode **le moins récemment
joué** : la fin du parcours devient une rotation de révision, pas une impasse.

## L'écran d'accueil est une carte où l'on se déplace

Au lancement, le partenaire est **posé sur la prochaine route**, sur la carte
de sa région : JOUER est déjà là. **Au retour d'une séance** (route, combat,
Tour, mini-jeu, rencontre), la carte reste sur la région d'où il est parti — et
à sa case s'il n'y a pas de route à faire ici. Elle ne passe d'elle-même à la
région suivante que lorsque toutes les routes de celle-ci sont faites et que
la suivante attend juste après (en mode libre, la « prochaine route » est la
route 1 : chaque retour renvoyait à la première région). On se déplace :

- au **clavier** : flèches, ZQSD ou WASD ; Entrée ou Espace = le bouton ;
- au **doigt** : toucher une case, le partenaire y marche tout seul (chemin le
  plus court) ; sur tablette, une croix directionnelle en plus (masquée sur
  téléphone, où l'on touche la carte).

### Une carte par région

**Chaque région a sa propre carte** (`OW_REGIONS` dans `web/overworld.js`,
13 × 9, tournée d'un quart de tour sur un écran en portrait), son obstacle et
ses couleurs (`.look-*` dans `styles.css`) :

| Région | Obstacle | La carte |
|---|---|---|
| Forêt Émeraude | 🌲 | sentier qui serpente dans une forêt dense, un ruisseau, herbes à gauche |
| Colline Azur | 🌳 | prairie ouverte, un lac en haut à droite |
| Grotte Violette | ⛰️ | galeries en zigzag, lac souterrain, mousse |
| Volcan Rougeoyant | 🌋 | chemin en U autour d'un lac de **lave** |
| Côte Saphir | 🌴 | promenade de plage, les routes au bout des pontons dans la mer |
| Vallée Arc-en-ciel | 🌸 | rivière coupée par un pont, prés fleuris |
| Îles du Levant | 🎋 | quatre îles reliées par des ponts |
| Îles du Couchant | 🌵 | dunes autour d'un lagon violet |
| Plateau de la Ligue | 🗿 | grande allée de pierre, les portes du Conseil de part et d'autre, le Champion au bout |
| Terres Nouvelles | 🌻 | champs en S |
| Failles Ultra | 🌌 | plateformes flottantes dans un **vide étoilé** |
| Académie Pokémon | 🏫 | campus, cour carrée autour d'une fontaine |

`internal/curriculum/overworld_test.go` lit ces cartes et vérifie chacune :
taille, une seule fois chaque lieu, tout atteignable depuis l'entrée, **les
routes rencontrées dans l'ordre** en marchant (1 avant 2…), aucun Dresseur à
côté d'une sortie ou du professeur (sa boîte de dialogue passerait avant), au
moins 4 cases d'herbes — et deux régions n'ont jamais la même carte ni le même
obstacle. Les emplacements de route sans épisode deviennent du chemin.

Sur chaque carte, les routes sont des Poké Balls le
long du chemin (le Pokémon à gagner au-dessus), **les hautes herbes** font
surgir un Pokémon sauvage en marchant (1 pas sur 7, s'il reste des Poké
Balls), et trois lieux s'ouvrent quand on marche dessus : 🗼 la Tour, 🎮 les
mini-jeux, 📘 le Pokédex. Les flèches au bord mènent aux régions voisines ; une
région dont la première route n'est pas débloquée reste fermée. Marcher sur
quelque chose ouvre une boîte de dialogue **lue à voix haute**, avec un seul
gros bouton. L'ancienne page d'accueil (liste des routes, badges) est le menu
☰.

## Le monde Pokémon : partenaire, Poké Balls, hautes herbes

- **Le partenaire.** Au premier lancement : « Choisis ton premier Pokémon ! »
  — Bulbizarre, Salamèche, Carapuce ou Pikachu (`data/partners.json` : les
  lignées et les niveaux d'évolution). Il gagne de l'**expérience à chaque
  lecture** (bonnes réponses d'une séance + bonus d'étoiles, points de la Tour,
  mini-jeux, captures), monte de niveau (N.5 → N.100) et **évolue à l'écran**
  (N.16 et N.32 ; Raichu au N.24) : « Quoi ? Salamèche évolue ! ». C'est lui
  qui combat à ses côtés et le suit d'écran en écran. Une sauvegarde qui
  existait avant reçoit une avance d'XP (8 par épisode terminé, juste sous la
  première évolution). Courbe : `curriculum.XPToNext`.
- **Les Poké Balls.** Gagnées en lisant l'aventure (2 + une par étoile à la fin
  d'une route, 2 par étage de la Tour, 1 par record battu), plus **le cadeau du
  jour** (3 Poké Balls, une fois par jour : revenir demain, c'est l'habitude
  qui apprend à lire). 5 sont offertes avec le partenaire.
- **🌿 Les hautes herbes.** On touche les herbes, un Pokémon sauvage surgit :
  pour l'attraper, **lire son nom** en remettant ses syllabes dans l'ordre (avec
  des intruses), puis lancer la Poké Ball — une secousse si le nom a été lu
  sans faute, trois sinon. 1 chance sur 16 d'un chromatique. Ils vont dans le
  **Pokédex sauvage** (second onglet du Pokédex), à côté du Pokédex de
  l'aventure, qui reste celui qu'on complète en lisant le cours.

### La pêche

**🎣 La canne à pêche se mérite.** Un **Pêcheur** (👴, case `K`) se tient près
du ponton de la Côte Saphir : « Lis une route presque sans faute, et ma canne
à pêche est à toi ! ». Elle est gagnée dès qu'une route, de la première de la
Côte Saphir (`curriculum.RodFrom` = ep13) à la fin, est terminée avec **au
moins 4 étoiles** (`RodStars` : au moins 90 % de bonnes réponses du premier
coup) ; l'écran de fin le fête une fois (`rod_won`). Rien n'est stocké : la
canne se déduit des meilleures étoiles (`HasRod`), donc un épisode remis à
zéro dans l'admin peut la reprendre.

**Pêcher** : avec la canne, toucher l'eau sur la carte — le partenaire marche
jusqu'à la rive, puis l'écran de pêche : le flotteur attend, plonge (« Ça mord
! Touche vite ! »), et un toucher dans les 3,5 s sort un Pokémon de l'eau ; trop
tard, il repart et le flotteur se relance. Ensuite, la même lecture que dans les
hautes herbes (remettre les syllabes du nom, lancer la Poké Ball). Sans canne,
toucher l'eau dit où la trouver. Pas de pêche dans la lave du volcan, le vide
des Failles Ultra ni la fontaine de l'Académie (`OW_NO_FISH`).

**Les Pokémon de l'eau** : les entrées d'`arcade.json` marquées `"water": true`
ne vivent **que dans l'eau** — jamais dans les herbes. Seize Pokémon d'eau des
générations 1–2 qui étaient dans les herbes (Stari, Loupio, Kabuto, Démanta…)
et six nouveaux, pêche seulement : Barpau, Sancoki, Flotajou, Bargantua,
Obalie, Mateloutre. Mêmes règles de lecture que les herbes (`validateWild`,
`WildUnlock`) — Tritosor (`s` entre voyelles), Luminéon (`in` glouton),
Mamanbo et Clamiral (`am` glouton) ont été refusés par le validateur. Ils
apparaissent dans le Pokédex sauvage avec un 🎣. Le serveur refuse d'attraper
un Pokémon de l'eau sans la canne.

### Qui vit dans les hautes herbes

Les Pokémon d'`arcade.json` qui ont un champ `syllables`. Un nom n'entre dans
les herbes qu'à partir de l'épisode où **tous ses sons sont enseignés et où la
segmentation donne la vraie lecture** (`WildUnlock`) — sinon « Loupio » serait
« lisible » dès la route 1 en o-u. Résultat : 5 Pokémon après la route 1, puis
de nouveaux à presque chaque route, 101 au total. Plus il apprend de sons, plus
il en trouve.

Les noms qui se liraient faux ne reçoivent pas de syllabes : finale muette
(Feunard, Tarpaud), `am`/`em` lus /ɑ̃/ (Alakazam, Empiflor), `ill` lu /ij/
(Marill), nom anglais (Snubbull, Steelix), `cen` avalé par `ce` (Héricendre).
`pokecontent check` refuse en plus, pour tout Pokémon sauvage, une nasale
suivie d'une voyelle, un `er` au milieu du mot, un `s` entre voyelles, un
`c`/`g` devant e/i et un `y` après voyelle.

## Les raisons de revenir demain

- **Niveau de Dresseur.** Tout ce qu'il fait rapporte de l'expérience au
  Dresseur lui-même (lire, attraper, finir une mission, battre un Dresseur) :
  des niveaux (`🎖 N.7` sur la carte), des titres (Dresseur débutant →
  apprenti → confirmé → expert → Champion d'arène → Maître Pokémon), et à
  chaque niveau 2 Poké Balls — tous les 5 niveaux, une Super Ball et une baie
  en plus.
- **L'équipe (👥).** N'importe quel Pokémon attrapé peut devenir le partenaire ;
  **chacun garde son niveau** et évolue à l'écran en l'atteignant
  (`data/evolutions.json` : 47 lignées tirées du roster existant, ex. Roucool →
  Roucoups → Roucarnage). La forme évoluée **entre dans sa collection** (le
  Pokédex, ou le Pokédex sauvage) : un autre chemin pour les compléter.
- **Le Pokédex dit où trouver chacun.** Toucher une silhouette : « Il t'attend
  sur la route 7 ! », « … à l'étage 12 de la Tour de Combat ! », « Fais
  évoluer un de tes Pokémon pour l'avoir ! » (`curriculum.Where`). Les 192 sont
  attrapables : routes, arènes, Tour (étages 2 à 115), examens surprises — `pokecontent check`
  refuse un Pokémon qui ne l'est nulle part. **Tous les 10 Pokémon**, une
  récompense (Super Ball + 2 Poké Balls).
- **Missions du jour (📜).** Trois par jour, tirées de sept (attraper, lire dix
  mots sans faute, finir une route, battre un Dresseur, trouver un objet,
  nourrir le partenaire, jouer à un mini-jeu), lues par la voix, avec leur
  récompense.
- **Dresseurs sur la carte.** Deux par région ; passer à côté déclenche le défi
  — un combat éclair de trois mots, un adversaire différent chaque jour, une
  revanche le lendemain.
- **Objets cachés.** Deux ✨ par région et par jour : Poké Balls, **Super Ball**
  (+20 % de chances de capture, proposée au lancer), Super Bonbon (+30 XP),
  baie.
- **Un partenaire vivant.** Le toucher : il saute et dit son nom. Une fois par
  jour, il a faim (🍓 au-dessus de sa tête) : une baie le rend heureux et un
  peu plus fort.
- **Le combat de fin de leçon** : le partenaire lance une vraie attaque de son
  type (« Salamèche utilise Flammèche ! », le projectile traverse l'arène),
  bonus *Super rapide !* pour un mot bâti vite et sans faute, K.O. final mis en
  scène, et un rythme de combat (original) qui accélère quand l'adversaire
  faiblit.

Tout ce qui est quotidien vit dans `progress.State.Today` et se renouvelle au
premier événement d'un nouveau jour.

## Mini-jeux — chronos, scores et records

Un coin **🎮 Mini-jeux** sur l'accueil (et à la fin de chaque séance) : dix
jeux, **débloqués un à un dans l'aventure** (`curriculum.GameUnlocks`) —
Memory après la route 1, la chasse et **Mon livre** après la 2, Lis et attrape après la 3,
l'oreille fine après l'Arène de Glace, l'atelier après la route 5, Pokémon
mystère après l'Arène de l'Orage, les paires après la route 8, le tambour
après l'Arène Psy, le son manquant après l'Arène du Volcan. Un jeu fermé
montre un 🔒 et se dit « plus loin dans l'aventure » ; l'écran de fin
annonce le jeu qui vient de s'ouvrir. Le mode libre de l'admin ouvre tout.
**Les tickets 🎟️** : Memory et La chasse sont les deux jeux sans lecture ;
une partie coûte un ticket, et les tickets se gagnent **en lisant** — une
route terminée, un étage de la Tour, un examen réussi, un livre de « Mon
livre » : un ticket chacun. Chaque sauvegarde en reçoit 3 au départ. Sans
ticket, le jeu le dit : « Lis une route ou ton livre pour en gagner. »
Chacun a **un record enregistré à battre** — nouveau
record = feux d'artifice. Deux sont de la pure récompense, deux sont de la
lecture déguisée :

- **⚒️ L'atelier des mots** : le mécanisme du combat en course contre la
  montre — reconstruire un maximum de mots en 90 secondes (alimente la
  répétition espacée).
- **📖 Lis et attrape** : lire le mot, toucher la bonne image, 60 secondes —
  chaque réponse alimente la répétition espacée.
- **👂 L'oreille fine** : entendre la syllabe, la toucher parmi quatre,
  60 secondes — pareil.
- **❓ Pokémon mystère** : une silhouette, quatre noms écrits — lire les noms
  de Pokémon, le fantasme du jeu distillé.
- **🔤 Les paires mot-image** : un memory où une carte sur deux est le mot
  ÉCRIT et l'autre son image — lire pour apparier, le record est le temps.
- **🃏 Memory Pokémon** : des plateaux de 3, 4, 6 puis 8 paires de Pokémon, le
  record est le temps total.
- **🌿 La chasse** : 45 secondes, les Pokémon surgissent des hautes herbes de
  plus en plus vite.

- **🥁 Le tambour des syllabes** : entendre le mot, taper sur le tambour du
  bon nombre de syllabes ; la réponse est rejouée syllabe par syllabe, un coup
  de tambour chacune. Les mots finis par une syllabe muette (« Salamèche » :
  trois syllabes à l'oral, quatre à l'écrit) en sont exclus.
- **📚 Mon livre** : cinq phrases déjà lues dans l'aventure (les plus
  récentes d'abord), une par page avec son image. Il lit à voix haute (chaque
  mot se touche), puis « J'AI LU ! » relit la page en karaoké. Relire, c'est ce
  qui rend la lecture fluide. Le score est le nombre de mots lus.
- **🧩 Le son manquant** : un mot avec un trou, trois graphèmes à proposer
  (chacun se fait entendre quand on le touche). Jamais deux homophones parmi
  les choix (`o`/`au`/`eau`, `an`/`en`…) : il aurait raison et on lui dirait
  non.

Les records se gèrent dans l'admin (onglet Réglages) : un tableau des dix mini-jeux, joués ou non, avec leur meilleur score et son unité (le temps pour Memory et les paires, où le plus petit gagne ; les mots pour Mon livre), un 🔒 pour ceux qui ne sont pas encore ouverts, et **un bouton « Remettre à zéro » par jeu** — plus un pour tous, avec confirmation. Chaque partie
rapporte aussi de l'expérience au partenaire.

### Mini-jeux : des niveaux

Chaque partie commence au **niveau 1** et monte jusqu'au 5 : puce « Niveau N »
en haut, bannière « NIVEAU 2 ! » et la voix dit « Niveau suivant ! ». Un point
vaut le niveau (×2/×3 du combo en plus) : le score dit jusqu'où il est monté,
et l'écran de fin affiche le niveau atteint. Mécanique commune dans `Arcade`
(`levelUp`, `advance` = un niveau toutes les N bonnes réponses du premier
coup, `reach` = le niveau imposé par la manche) ; chaque jeu décide ce qui
durcit :

| Jeu | Monte | Ce qui durcit |
|---|---|---|
| 📖 Lis et attrape | 4 bonnes réponses | 3 → 4 images ; mots de 2, puis 3 syllabes, puis 3 et plus. Mauvaises images toujours « qui se ressemblent » |
| ❓ Pokémon mystère | 4 | 3 → 4 noms (toujours qui se ressemblent) ; ombre plus petite au niveau 4 |
| 👂 L'oreille fine | 4 | 3 → 4 syllabes ; une voisine piège (ba/da) un tour sur deux au niveau 2, à chaque tour au 3, deux au 4 |
| ⚒️ L'atelier | 3 mots | mots de 2 → 3 → 3+ syllabes ; 1 → 3 syllabes intruses, qui commencent comme celles du mot ; niveau 5 : mot caché, dicté |
| 🌿 La chasse | toutes les 9 s | plus de Voltorbe (1/7 → 1/3), en plus de l'accélération |
| 🃏 Memory | chaque plateau | 3, 4, 6, 8 paires |
| 🔤 Les paires | chaque plateau | 3, 4, 5 mots ; dès le 2ᵉ, des mots qui commencent pareil |
| 🥁 Le tambour | toutes les 2 manches | les 10 mots du moins au plus de syllabes ; dès le niveau 4, le mot n'est plus écrit (on compte à l'oreille) |
| 🧩 Le son manquant | toutes les 2 manches | mots du plus court au plus long ; trou de consonne à 2 choix, puis 3, puis voyelle, puis son à deux lettres (ou, on, ch…), 4 choix au niveau 5 |
| 📚 Mon livre | chaque page | pages de la plus courte à la plus longue |

Memory et les paires comptaient un seul plateau : leurs anciens records (un
temps sur 6 paires) sont effacés une fois à l'ouverture de la sauvegarde
(`State.BoardGames`), sinon les 4 plateaux ne pourraient jamais les battre.
Les autres records restent ; les points pondérés par le niveau les rendent
plus faciles à battre une première fois.

`Arcade.screen()` n'arrête plus le jeu précédent (c'est `play()` qui le
fait) : les jeux chronométrés lancent leur compte à rebours *avant* de
construire leur écran, et `screen()` l'arrêtait aussitôt.

Le matériel vient de `GET /api/arcade` : les jeux de lecture n'utilisent que ce
qui a été appris, mais les trois jeux Pokémon (mystère, memory, chasse) piochent
dans **tout le roster** — les Pokémon du cours, attrapés ou non, plus
`data/arcade.json` (le reste des générations 1 et 2, noms français de PokeAPI).
Avec seulement trois captures, ils se répétaient en boucle. Ces Pokémon-là
n'entrent jamais au Pokédex ; leur artwork est réduit à 240 px par
`pokecontent sprites`. Un Pokémon ajouté au cours doit être retiré
d'`arcade.json` (`pokecontent check` le signale). Les records vivent dans la
sauvegarde (`POST /api/records`).

### La phrase : « Lis la phrase »

Comme au CP, en trois temps :

1. **Lire à voix haute** — la phrase en grand, chaque mot touchable pour
   l'entendre s'il bloque, puis « J'AI LU ! ».
2. **Comprendre** — « Quelle image va avec la phrase ? », trois images. Les
   mauvaises ne montrent jamais un mot de la phrase (avec « Otaria utilise
   Morsure. », l'icône Morsure serait aussi une bonne réponse) et commencent
   comme le sujet (Abo / Abra) : la première lettre ne suffit pas. Une erreur
   dit « Presque ! Relis la phrase. », sans la lire.
3. **La relecture** — seulement après la bonne image : la voix relit la phrase
   et **chaque mot s'allume** au moment où elle le prononce (karaoké calé sur
   l'horloge du clip) : il se corrige tout seul. Placée avant la question,
   elle donnait la réponse à l'oreille.

L'ancien « Construis la phrase » (remettre les mots mélangés dans l'ordre)
demandait de deviner la syntaxe sans modèle sous les yeux : ce n'était pas de
la lecture.

### La prononciation des syllabes : `data/pronunciation.json`

La synthèse épelle un morceau nu comme « ny » et lit « cher » comme le mot
/chair/ au lieu du son /ché/. Le fichier `data/pronunciation.json` donne la
respélisation **pour la voix seulement** (l'enfant voit toujours la vraie
graphie), appliquée à la génération **et** au serveur — les deux restent
d'accord. Une syllabe sonne mal ? Ajoute une ligne, `make content`, terminé.
Et pour une prononciation parfaite : le studio voix, ta voix gagne toujours.

---

## Déploiement

En local, sur le réseau de la maison : `make run`, et c'est tout.

En ligne, à côté d'un site existant, sous un sous-chemin :

```bash
make content                        # la voix (générée sur Mac) doit exister avant le build
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

- **La voix** est générée sur Mac (voir « La voix ») : elle voyage dans le contexte de
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
