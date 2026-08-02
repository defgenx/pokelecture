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

## 1. Générer la voix (sur le Mac)

Le `.m4a` est produit par `say`, qui n'existe que sur macOS : il doit voyager
dans le contexte de build.

```bash
cd pokelecture
make content          # valide, génère la voix, récupère les sprites
```

Sans cette étape l'image se construit quand même : le jeu retombe sur la voix
du navigateur.

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
      POKELECTURE_NAME: Dresseur
    volumes:
      - pokelecture_var:/app/var
      - pokelecture_voice:/app/web/audio/recorded

volumes:
  pokelecture_var:
  pokelecture_voice:
```

Et `pokelecture` dans le `depends_on` de nginx.

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
```

## Notes

- **Le studio voix (`/pokelecture/parent.html`) sera accessible publiquement.**
  Le micro exige un contexte sécurisé, ce que HTTPS fournit — donc n'importe qui
  pourra écraser la voix du jeu. Si le site est indexé, protège-le : un
  `location = /pokelecture/parent.html { auth_basic … }` suffit, ou retire la
  page de l'image en production.
- Le Pokédex, la progression et les étoiles sont **globaux** : une seule
  sauvegarde, pas de comptes. C'est voulu (un seul enfant), mais ça veut dire que
  n'importe quel visiteur fait avancer sa progression. Même remarque : si l'URL
  circule, mets un `auth_basic` sur tout `/pokelecture/`.
- Les sprites sont téléchargés au build depuis PokeAPI, jamais commités.
