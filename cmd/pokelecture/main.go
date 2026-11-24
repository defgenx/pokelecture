// Command pokelecture serves the reading game on the home network.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/api"
	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
)

func main() {
	addr := flag.String("addr", envOr("POKELECTURE_ADDR", ":8080"), "adresse d'écoute")
	root := flag.String("root", envOr("POKELECTURE_ROOT", "."), "racine du projet (contient data/ et web/)")
	// Empty means "not configured": keep whatever the savegame holds, and fall
	// back to progress.DefaultName on a fresh one.
	name := flag.String("name", envOr("POKELECTURE_NAME", ""), "prénom de l'enfant (défaut: celui de la sauvegarde)")
	base := flag.String("base", envOr("POKELECTURE_BASE", "/"), "chemin de montage, ex. /pokelecture/")
	// A flag, not an env var: a deployment's env_file must never be able to turn it on.
	localAdmin := flag.Bool("local-admin", false, "ouvre l'admin sans connexion Google, depuis cet ordinateur seulement (localhost)")
	flag.Parse()

	mount := normalizeBase(*base)

	cur, err := curriculum.Load(filepath.Join(*root, "data"))
	if err != nil {
		log.Fatal(err)
	}
	if errs := cur.Validate(); len(errs) > 0 {
		for _, e := range errs {
			log.Printf("contenu: %v", e)
		}
		log.Fatalf("%d problème(s) de contenu — corrige data/ puis relance", len(errs))
	}

	// One save per player in var/players/; a save from before players becomes
	// the first one (-name / POKELECTURE_NAME applies while there is only one).
	players, err := progress.OpenRegistry(filepath.Join(*root, "var"), *name)
	if err != nil {
		log.Fatal(err)
	}

	// Google sign-in for the parent pages (admin, voice studio): on as soon
	// as a client id is set. README, « Administration ».
	auth, err := api.NewAuth(os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("POKELECTURE_ADMINS"), os.Getenv("POKELECTURE_URL"), os.Getenv("POKELECTURE_SESSION_KEY"))
	if err != nil {
		log.Fatalf("connexion Google : %v", err)
	}
	switch {
	case auth == nil:
		log.Printf("admin et studio voix FERMÉS : définis GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET et POKELECTURE_ADMINS pour y accéder")
	case os.Getenv("POKELECTURE_SESSION_KEY") == "":
		log.Printf("connexion Google active (%d adresse(s)) — POKELECTURE_SESSION_KEY vide : reconnexion à chaque redémarrage", len(auth.Allowed))
	default:
		log.Printf("connexion Google active (%d adresse(s) autorisée(s))", len(auth.Allowed))
	}

	if *localAdmin {
		if u := os.Getenv("POKELECTURE_URL"); u != "" && !isLocalURL(u) {
			log.Fatalf("-local-admin refusé : POKELECTURE_URL (%q) n'est pas une adresse locale — c'est un serveur en ligne", u)
		}
		log.Printf("admin et studio voix OUVERTS sans connexion depuis cet ordinateur (http://localhost%s/admin) — -local-admin", *addr)
	}

	handler := api.NewPlayers(cur, players, filepath.Join(*root, "web"), mount).WithAuth(auth).WithLocalAdmin(*localAdmin).Routes()

	// Mounted under a sub-path, strip it before the app's own mux sees the
	// request. Go's ServeMux also redirects /pokelecture → /pokelecture/ for us,
	// which is what makes the relative URLs resolve correctly.
	if mount != "/" {
		outer := http.NewServeMux()
		outer.Handle(mount, http.StripPrefix(strings.TrimSuffix(mount, "/"), handler))
		handler = outer
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("\n  Pokélecture — %s, %d épisode(s), %d pokémon (monté sur %s)\n",
		playerNames(players), len(cur.Episodes), len(cur.Pokemon), mount)
	for _, u := range urls(*addr) {
		fmt.Printf("  ▸ %s%s\n", u, strings.TrimPrefix(mount, "/"))
	}
	fmt.Printf("\n  Ouvre l'adresse 192.168.x.x sur la tablette, puis « Ajouter à l'écran d'accueil ».\n\n")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	_ = os.Stdout.Sync()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// normalizeBase turns "", "pokelecture", "/pokelecture" all into "/pokelecture/".
// Every relative URL in the frontend resolves against this, so it has to end in a
// slash or the last path segment would be dropped.
func normalizeBase(b string) string {
	b = strings.TrimSpace(b)
	if b == "" || b == "/" {
		return "/"
	}
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	if !strings.HasSuffix(b, "/") {
		b += "/"
	}
	return b
}

// urls lists every address the tablet could use, so there is nothing to guess.
func urls(addr string) []string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		port = "8080"
	}
	out := []string{"http://localhost:" + port}

	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			out = append(out, fmt.Sprintf("http://%s:%s", ipnet.IP.String(), port))
		}
	}
	return out
}

func playerNames(r *progress.Registry) string {
	var names []string
	for _, p := range r.List() {
		names = append(names, p.Name)
	}
	return strings.Join(names, ", ")
}

// isLocalURL: a POKELECTURE_URL on this computer (http://localhost:8080/…).
func isLocalURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}
