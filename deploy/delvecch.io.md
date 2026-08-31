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

Le `.m4a` est produit par `say`, qui n'existe que sur macOS : il doit voyager
dans le contexte de build. Or le serveur build depuis un clone du dépôt — donc
`web/audio/` est versionné, et une génération non committée n'atteint jamais la
tablette.

```bash
cd pokelecture
make content          # valide, génère la voix, récupère les sprites
git add web/audio && git commit -m "chore: régénère la voix" && git push
```

Sans cette étape l'image se construit quand même, mais le jeu part **muet** :
il retombe sur la voix du navigateur, ce que Chrome sur Android ne fournit que
si une voix française est installée sur l'appareil. Sur une tablette Android,
« pas de `.m4a` » veut donc dire « pas de son du tout ».

Vérifier, après déploiement, qu'un fichier répond bien :

```bash
curl -sI https://delvecch.io/pokelecture/audio/bulbizarre-8a23d0.m4a | head -3
# HTTP/2 200 · content-type: audio/mp4
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

Le jeu reste ouvert (pas de mot de passe devant un enfant de cinq ans), mais le
studio et l'administration sont publics sinon : n'importe qui pourrait écraser
ta voix — ou **effacer la sauvegarde** via `api/admin/reset`. Protéger les pages
seules ne suffirait pas — elles écrivent via `api/record/` et `api/admin/`,
qu'on atteint directement en `curl` sans jamais charger la page. Il faut donc
tous ces emplacements, **avant** `location /pokelecture/` :

```nginx
    location = /pokelecture/parent.html {
        include /etc/nginx/conf.d/studio-auth.inc;
    }

    location = /pokelecture/admin.html {
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
`parent.html` ou `admin.html` quand la page appelle ensuite ses endpoints.

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

- Le studio voix **et l'administration** sont derrière `auth_basic` (étape
  4 bis) : pages **et** endpoints d'écriture — `api/admin/reset` efface toute la
  sauvegarde. Le micro exige un contexte sécurisé, ce que HTTPS fournit.
- Le Pokédex, la progression et les étoiles sont **globaux** : une seule
  sauvegarde, pas de comptes. C'est voulu (un seul enfant), mais ça veut dire que
  n'importe quel visiteur fait avancer sa progression. Même remarque : si l'URL
  circule, mets un `auth_basic` sur tout `/pokelecture/`.
- Les sprites sont téléchargés au build depuis PokeAPI, jamais commités.
