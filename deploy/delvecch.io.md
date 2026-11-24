# Déployer Pokélecture sous `delvecch.io/pokelecture/`

Le jeu tourne à côté du site, dans le même `docker-compose`, derrière le nginx
qui termine déjà le TLS.

## Pourquoi ça marche sous un sous-chemin

Le serveur accepte `-base` / `POKELECTURE_BASE` (ex. `/pokelecture/`) et :

1. réécrit `<base href="__BASE__">` dans `index.html`, `parent.html` et le
   manifest ;
2. retire le préfixe avant son propre routeur (`http.StripPrefix`).

**Toutes** les URLs émises par le serveur et le frontend sont relatives
(`audio/…`, `sprites/…`, `api/…`), donc elles se résolvent contre ce `<base
href>`. Conséquence pratique : nginx n'a **rien** à réécrire — il passe le
chemin complet tel quel, ce qui évite la classe de bugs habituelle des
sous-chemins (redirections cassées, assets qui remontent à la racine).

Le même binaire tourne donc sans changement à la racine en local et sous
`/pokelecture/` en production.

## 1. Générer la voix (sur le Mac), puis **la committer**

La voix (MP3, ElevenLabs, payante) est générée une fois sur le Mac : elle doit
voyager dans le contexte de build. Or le serveur build depuis un clone du dépôt — donc
`web/audio/` est versionné, et une génération non committée n'atteint jamais la
tablette.

```bash
cd pokelecture
make content          # valide, génère la voix, récupère les sprites
git add web/audio && git commit -m "chore: régénère la voix" && git push
```

Sans cette étape l'image se construit quand même, mais le jeu part **muet** :
il n'utilise jamais de voix de synthèse — « pas de `.mp3` » veut dire « pas de
son du tout ».

Vérifier, après déploiement, qu'un fichier répond bien :

```bash
curl -sI https://delvecch.io/pokelecture/audio/bulbizarre-8a23d0.mp3 | head -3
# HTTP/2 200 · content-type: audio/mpeg
```

## 2. Cloner le dépôt à côté du site

Sur le serveur, à côté de `delvecch.io/` :

```bash
git clone git@github.com:defgenx/pokelecture.git
```

## 3. Ajouter le service au `docker-compose.yml` de delvecch.io

```yaml
  pokelecture:
    build: ../pokelecture
    image: pokelecture:1.0.0
    container_name: pokelecture
    restart: unless-stopped
    expose:
      - "8080"
    environment:
      POKELECTURE_BASE: /pokelecture/
      POKELECTURE_NAME: ${POKELECTURE_NAME:-}
      # Connexion Google de l'admin et du studio (étape 4 bis) — valeurs dans
      # le .env non versionné à côté de ce compose.
      GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID:-}
      GOOGLE_CLIENT_SECRET: ${GOOGLE_CLIENT_SECRET:-}
      POKELECTURE_ADMINS: ${POKELECTURE_ADMINS:-}
      POKELECTURE_SESSION_KEY: ${POKELECTURE_SESSION_KEY:-}
      POKELECTURE_URL: https://delvecch.io/pokelecture/
    volumes:
      - pokelecture_var:/app/var
      - pokelecture_voice:/app/web/audio/recorded

volumes:
  pokelecture_var:
  pokelecture_voice:
```

Et `pokelecture` dans le `depends_on` de nginx.

`POKELECTURE_NAME` est lu au démarrage et **écrase** le prénom déjà dans la
sauvegarde : renommer l'enfant ne coûte qu'un redémarrage, et la progression est
conservée. Laissé vide, la sauvegarde garde son prénom (`Dresseur` sur une
sauvegarde neuve). Mets-le dans un `.env` non versionné à côté du
`docker-compose.yml` — le dépôt est public.

## 4. Ajouter le `location` nginx

Dans le bloc `server { listen 443 ssl; }`, **avant** le `location /` :

```nginx
    # Pokélecture. proxy_pass sans URI => nginx transmet /pokelecture/... tel
    # quel, et l'app retire le préfixe elle-même (POKELECTURE_BASE).
    location /pokelecture/ {
        proxy_pass http://pokelecture:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Les enregistrements du studio voix montent jusqu'à ~8 Mo.
        client_max_body_size 12m;
    }

    location = /pokelecture {
        return 301 /pokelecture/;
    }
```

## 4 bis. Protéger le studio voix et l'administration

**Recommandé : la connexion Google.** L'app s'en charge elle-même (pages et
endpoints), seules les adresses de `POKELECTURE_ADMINS` entrent, et sans
configuration tout le côté parent est **fermé**.

1. Console Google (README, « Se connecter avec Google ») : client OAuth *Application
   Web*, URI de redirection **`https://delvecch.io/pokelecture/auth/callback`**,
   écran de consentement en mode *Test* avec tes adresses comme utilisateurs test.
2. Dans le `.env` à côté du `docker-compose.yml` de delvecch.io (jamais
   versionné, `chmod 600`) :

   ```bash
   GOOGLE_CLIENT_ID=…apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=…
   POKELECTURE_ADMINS=toi@gmail.com
   POKELECTURE_SESSION_KEY=…        # openssl rand -hex 32
   ```

3. Le service passe ces variables (étape 3) et fixe
   `POKELECTURE_URL=https://delvecch.io/pokelecture/` : l'URL de rappel est alors
   exacte et aucun en-tête `X-Forwarded-*` n'est cru.
4. Les `location` `auth_basic` ci-dessous deviennent inutiles : retire-les une
   fois la connexion Google vérifiée (les garder impose deux connexions, et
   `/pokelecture/auth/` ne doit **pas** être derrière l'`auth_basic`).

Vérifier après `docker compose up -d pokelecture` :

```bash
docker compose logs pokelecture | grep connexion     # « connexion Google active (1 adresse(s) autorisée(s)) »
curl -s -o /dev/null -w '%{http_code}\n' https://delvecch.io/pokelecture/admin
# 401 : la page avec le bouton « Se connecter avec Google »
curl -s -o /dev/null -w '%{http_code}\n' -X POST https://delvecch.io/pokelecture/api/admin/reset   # 401
```

Puis ouvrir `https://delvecch.io/pokelecture/admin` : le bouton, Google, puis l'admin
(le studio voix : `/pokelecture/studio`).

**Variante sans Google : `auth_basic` dans nginx.** Le jeu reste ouvert (pas de mot de passe devant un enfant de cinq ans), mais le
studio et l'administration sont publics sinon : n'importe qui pourrait écraser
ta voix — ou **effacer la sauvegarde** via `api/admin/reset`. Protéger les pages
seules ne suffirait pas — elles écrivent via `api/record/` et `api/admin/`,
qu'on atteint directement en `curl` sans jamais charger la page. Il faut donc
tous ces emplacements, **avant** `location /pokelecture/` :

```nginx
    location = /pokelecture/studio {
        include /etc/nginx/conf.d/studio-auth.inc;
    }

    location = /pokelecture/admin {
        include /etc/nginx/conf.d/studio-auth.inc;
    }

    location ^~ /pokelecture/api/record/ {
        include /etc/nginx/conf.d/studio-auth.inc;
    }

    location ^~ /pokelecture/api/admin/ {
        include /etc/nginx/conf.d/studio-auth.inc;
    }

    location = /pokelecture/api/texts {
        include /etc/nginx/conf.d/studio-auth.inc;
    }
```

`studio-auth.inc` porte l'`auth_basic`, le `proxy_pass` et le
`client_max_body_size 12m` (les enregistrements montent à ~8 Mo). Un seul realm
pour tous, afin que le navigateur réutilise les identifiants saisis sur
`/studio` ou `/admin` quand la page appelle ensuite ses endpoints. (Les
anciennes adresses `parent.html` et `admin.html` ne font plus que rediriger.)

Crée le fichier d'identifiants **sur le serveur** (jamais versionné) :

```bash
cd delvecch.io
htpasswd -Bc frontend/nginx/htpasswd studio    # ou: docker run --rm httpd htpasswd -Bn studio
```

Sans ce fichier nginx répond 500 sur le studio — ça échoue fermé, donc pas de
fuite, mais la page est cassée jusqu'à ce qu'il existe.

## 5. Lancer

```bash
cd delvecch.io
docker compose build pokelecture
docker compose up -d pokelecture
docker compose exec nginx nginx -s reload
```

Vérifier :

```bash
curl -sI https://delvecch.io/pokelecture/           # 200
curl -s  https://delvecch.io/pokelecture/ | grep base   # <base href="/pokelecture/">
curl -s  https://delvecch.io/pokelecture/api/state | head -c 200

# La voix : 200 et audio/mp4, sinon la tablette sera muette (voir étape 1).
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' \
  "https://delvecch.io/pokelecture/$(curl -s https://delvecch.io/pokelecture/api/state \
   | python3 -c 'import sys,json;print(json.load(sys.stdin)["pokedex"][0]["name_audio"])')"
```

## Notes

- Le studio voix **et l'administration** se protègent soit par la **connexion
  Google** (README, « Se connecter avec Google » : `GOOGLE_CLIENT_ID`,
  `POKELECTURE_ADMINS`, `POKELECTURE_URL=https://delvecch.io/pokelecture/`,
  URL de rappel `https://delvecch.io/pokelecture/auth/callback`), soit par
  l'`auth_basic` de l'étape 4 bis — pages **et** endpoints d'écriture :
  `api/admin/reset` efface toute la sauvegarde. Sans `GOOGLE_CLIENT_ID`,
  l'admin et le studio sont **fermés** (503/403) : avec Google, l'`auth_basic`
  du studio devient inutile. Passe les variables au conteneur (`GOOGLE_CLIENT_ID`,
  `GOOGLE_CLIENT_SECRET`, `POKELECTURE_ADMINS`, `POKELECTURE_SESSION_KEY`,
  `POKELECTURE_URL`) via un `.env` à côté du compose — jamais versionné. Le
  micro exige un contexte sécurisé, ce que HTTPS fournit.
- Le Pokédex, la progression et les étoiles sont **globaux** : une seule
  sauvegarde, pas de comptes. C'est voulu (un seul enfant), mais ça veut dire que
  n'importe quel visiteur fait avancer sa progression. Même remarque : si l'URL
  circule, mets un `auth_basic` sur tout `/pokelecture/`.
- Les sprites sont téléchargés au build depuis PokeAPI, jamais commités.
