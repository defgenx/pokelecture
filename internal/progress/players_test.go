package progress

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryMigratesTheOldSave(t *testing.T) {
	dir := t.TempDir()
	old, err := Open(filepath.Join(dir, "progress.json"), "Léo")
	if err != nil {
		t.Fatal(err)
	}
	old.ChoosePartner(4, 0)
	if err := old.Save(); err != nil {
		t.Fatal(err)
	}
	r, err := OpenRegistry(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	list := r.List()
	if len(list) != 1 || list[0].Name != "Léo" {
		t.Fatalf("l'ancienne sauvegarde doit devenir le premier joueur : %v", list)
	}
	st, _ := r.Get(list[0].ID)
	if st.Snapshot().Partner == nil {
		t.Fatal("la progression migrée a été perdue")
	}
	if _, err := os.Stat(filepath.Join(dir, "progress.json.before-players")); err != nil {
		t.Fatal("l'ancien fichier doit rester en copie")
	}
	// Reopening does not migrate twice.
	r2, _ := OpenRegistry(dir, "")
	if len(r2.List()) != 1 {
		t.Fatalf("seconde ouverture : %d joueur(s)", len(r2.List()))
	}
}

func TestRegistryPlayersAreSeparate(t *testing.T) {
	dir := t.TempDir()
	r, err := OpenRegistry(dir, "Léo")
	if err != nil {
		t.Fatal(err)
	}
	leo := r.List()[0].ID
	mia, err := r.Create("Mia")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.Get(leo)
	b, _ := r.Get(mia)
	a.ChoosePartner(4, 0)
	if b.Snapshot().Partner != nil {
		t.Fatal("le partenaire de Léo est apparu chez Mia")
	}
	if _, _, ok := r.Only(); ok {
		t.Fatal("deux joueurs : Only ne doit rien renvoyer")
	}
	if err := r.Delete(mia); err != nil {
		t.Fatal(err)
	}
	trash, _ := filepath.Glob(filepath.Join(dir, "players", "trash", mia+"-*.json"))
	if len(trash) != 1 {
		t.Fatal("un joueur supprimé doit partir à la corbeille, pas disparaître")
	}
	if err := r.Delete(leo); err == nil {
		t.Fatal("le dernier joueur ne doit pas pouvoir être supprimé")
	}
	if _, ok := r.Get("../../etc"); ok || validID("../x") || validID("") {
		t.Fatal("un identifiant de joueur ne doit jamais désigner un chemin")
	}
}
