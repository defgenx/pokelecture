package curriculum

import "fmt"

// Gigamax maps a species to the artwork id of its real Gigamax form (the
// PokeAPI "-gmax" forms). pokecontent sprites downloads these too. Every other
// boss Dynamaxes: same picture, giant, red aura. README « Gigamax et Téracristal ».
var Gigamax = map[int]int{
	3: 10195, 6: 10196, 9: 10197, 12: 10198, 25: 10199, 52: 10200,
	68: 10201, 94: 10202, 99: 10203, 131: 10204, 133: 10205, 143: 10206,
}

// GmaxArt is the picture of a species' Gigamax form, "" when it has none.
func GmaxArt(id int) string {
	if g, ok := Gigamax[id]; ok {
		return fmt.Sprintf("sprites/art/%d.png", g)
	}
	return ""
}

// TeraRegion is where the fights are against people — the Conseil and the
// Champion — so the partner can Téracristalliser, as against map trainers.
const TeraRegion = "Plateau de la Ligue"
