package actelyohub

import (
	"strings"
	"testing"
)

// Reproduit le bug rapporté : une petite page dont le NOM et le TITRE contiennent
// « copine » ne remontait pas pour la requête « copine Operator », noyée par de
// grosses pages qui répètent « Operator » des dizaines de fois. Le classement doit
// mettre en tête la page qui matche le PLUS de termes distincts (couverture),
// puis pondérer par la rareté.
func TestMemSearchCoverageBeatsFrequency(t *testing.T) {
	testHome(t)

	// Deux pages « bruyantes » bourrées de « Operator », sans « copine ».
	if err := MemAdd("business-actelyo.md", "# Business Operator\n"+strings.Repeat("Operator veut un business. ", 40)); err != nil {
		t.Fatal(err)
	}
	if err := MemAdd("guide-actelyohub.md", "# Guide ACTELYO INFERENCE HUB\n"+strings.Repeat("projet de Operator. ", 30)); err != nil {
		t.Fatal(err)
	}
	// La petite page cible : matche les DEUX termes de la requête.
	if err := MemAdd("profil-copine-actelyo.md", "# Profil : Copine de Operator\n- Prénom : Bao\n"); err != nil {
		t.Fatal(err)
	}

	hits := MemSearch("copine Operator", 8)
	if len(hits) == 0 {
		t.Fatal("aucun résultat pour « copine Operator »")
	}
	if hits[0].File != "profil-copine-actelyo.md" {
		var names []string
		for _, h := range hits {
			names = append(names, h.File)
		}
		t.Fatalf("la page copine devrait être 1re (matche les 2 termes), obtenu ordre : %v", names)
	}
}

// Une requête mono-terme rare doit privilégier un match dans le nom/titre.
func TestMemSearchNameTitleBoost(t *testing.T) {
	testHome(t)
	_ = MemAdd("notes-diverses.md", "# Notes\nUn jour Bao est passée dire bonjour, puis repartie.\n")
	_ = MemAdd("profil-copine.md", "# Profil : Copine\n- Prénom : Bao\n")
	hits := MemSearch("copine", 8)
	if len(hits) == 0 || hits[0].File != "profil-copine.md" {
		t.Fatalf("la page dont le nom/titre porte « copine » devrait être 1re, obtenu : %+v", hits)
	}
}
