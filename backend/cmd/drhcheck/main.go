// Commande de vérification : rejoue l'ingestion DRH complète (parse →
// rattachement → agrégats → persistance) sur une copie de la base, et imprime
// le rapport. Sert à valider un millésime avant de l'importer en production.
package main

import (
	"fmt"
	"os"
	"sort"

	"iss-dashboard-backend/internal/drh"
	"iss-dashboard-backend/internal/store"
	syncer "iss-dashboard-backend/internal/sync"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: drhcheck <copie-de-iss.db> <agents.csv> <correspondances.csv> [annee]")
		os.Exit(2)
	}
	st, err := store.New(os.Args[1])
	check(err)
	defer st.Close()

	f, err := os.Open(os.Args[3])
	check(err)
	corr, cerrs := drh.ParseCorrespondancesCSV(f)
	f.Close()
	for _, e := range cerrs {
		fmt.Printf("  correspondances ligne %d : %s\n", e.Line, e.Message)
	}
	check(st.ReplaceDrhCorrespondances(corr))

	annee := 2026
	if len(os.Args) > 4 {
		fmt.Sscanf(os.Args[4], "%d", &annee)
	}
	f, err = os.Open(os.Args[2])
	check(err)
	defer f.Close()

	res, err := syncer.RunDrhImport(st, f, syncer.DrhImportParams{
		Annee: annee, SourceFile: os.Args[2], ImportedBy: "drhcheck",
	})
	if err != nil {
		for i, e := range res.Erreurs {
			if i < 10 {
				fmt.Printf("  ligne %d : %s\n", e.Line, e.Message)
			}
		}
		check(err)
	}

	im, rep := res.Import, res.Report
	fmt.Printf("\n%s — %d correspondances, %d agents, %d ms\n", im.Label, len(corr), im.NAgents, res.DureeMs)
	fmt.Printf("  structure de soins      %6d  %5.1f %%  (%d structures couvertes)\n", rep.NStructure, pct(rep.NStructure, rep.NAgents), rep.NStructuresVues)
	fmt.Printf("  bureau de district      %6d  %5.1f %%\n", rep.NBureau, pct(rep.NBureau, rep.NAgents))
	fmt.Printf("  administration centrale %6d  %5.1f %%\n", rep.NCentrale, pct(rep.NCentrale, rep.NAgents))
	fmt.Printf("  non rattaché            %6d  %5.1f %%\n", rep.NNonRattache, pct(rep.NNonRattache, rep.NAgents))
	fmt.Printf("  => catégorisés : %.1f %%\n", rep.PctCategorise())

	var srcs []string
	for s := range rep.ParSource {
		srcs = append(srcs, s)
	}
	sort.Slice(srcs, func(i, j int) bool { return rep.ParSource[srcs[i]] > rep.ParSource[srcs[j]] })
	fmt.Printf("\n  par source :")
	for _, s := range srcs {
		fmt.Printf("  %s=%d", s, rep.ParSource[s])
	}
	fmt.Println()

	inconnus, err := st.GetDrhNonReconnus(im.ID)
	check(err)
	fmt.Printf("\n  libellés non reconnus (%d) :\n", len(inconnus))
	for i, u := range inconnus {
		if i >= 12 {
			break
		}
		fmt.Printf("    %4d agents  [%-14s] %s\n", u.NAgents, u.Prefecture, u.Libelle)
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erreur:", err)
		os.Exit(1)
	}
}
