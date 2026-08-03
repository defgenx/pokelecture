// Command pokelecture serves the reading game on the home network.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
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

	store, err := progress.Open(filepath.Join(*root, "var", "progress.json"), *name)
	if err != nil {
		log.Fatal(err)
	}

	handler := api.New(cur, store, filepath.Join(*root, "web"), mount).Routes()

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
		store.Name(), len(cur.Episodes), len(cur.Pokemon), mount)
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
