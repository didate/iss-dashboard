package sync

import (
	"fmt"
	"io"
	"log"
	"time"

	"iss-dashboard-backend/internal/drh"
	"iss-dashboard-backend/internal/store"
)

// DrhImportParams describes one personnel file import.
type DrhImportParams struct {
	Label       string
	Annee       int
	AgeRetraite int
	SourceFile  string
	ImportedBy  string
}

// DrhImportResult is what the admin screen shows back.
type DrhImportResult struct {
	Import  *store.DrhImport `json:"import"`
	Report  drh.Report       `json:"report"`
	Erreurs []drh.LineError  `json:"erreurs,omitempty"`
	DureeMs int64            `json:"duree_ms"`
}

// RunDrhImport is the single ingestion path for the DRH file: parse, attach to
// ISS facilities, aggregate, persist. The file itself is never stored — only
// the aggregated cells, which is what keeps the personnel data non-nominative.
//
// Strict: any rejected line aborts the import, so a malformed file cannot
// silently produce headcounts that are short of a few hundred agents.
func RunDrhImport(st *store.Store, r io.Reader, p DrhImportParams) (*DrhImportResult, error) {
	start := time.Now()

	rows, lineErrs := drh.ParseCSV(r)
	if len(lineErrs) > 0 {
		return &DrhImportResult{Erreurs: lineErrs}, fmt.Errorf("%d ligne(s) invalide(s), rien n'a été importé", len(lineErrs))
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("aucun agent dans le fichier")
	}

	structures, err := st.ListDrhStructures()
	if err != nil {
		return nil, fmt.Errorf("lecture des structures ISS : %w", err)
	}
	if len(structures) == 0 {
		return nil, fmt.Errorf("aucune structure ISS en base : lancez d'abord une synchronisation DHIS2")
	}
	corr, err := st.ListDrhCorrespondances()
	if err != nil {
		return nil, fmt.Errorf("lecture des correspondances : %w", err)
	}

	affs, report := drh.ResolveAll(rows, drh.NewResolver(structures, corr))
	opt := drh.Options{RefYear: p.Annee, RetirementAge: p.AgeRetraite}.Normalize(time.Now().Year())
	if p.Label == "" {
		p.Label = fmt.Sprintf("DRH/CNPS %d", opt.RefYear)
	}
	eff, pyr := drh.Aggregate(rows, affs, opt)

	log.Printf("[DRH] %s : %d agents, %d rattachés à %d structures, %d bureaux, %d centrale, %d non rattachés (%.1f %% catégorisés)",
		p.Label, report.NAgents, report.NStructure, report.NStructuresVues, report.NBureau, report.NCentrale,
		report.NNonRattache, report.PctCategorise())

	im, err := st.SaveDrhImport(store.DrhImport{
		Label:        p.Label,
		Annee:        opt.RefYear,
		AgeRetraite:  opt.RetirementAge,
		NAgents:      report.NAgents,
		NStructure:   report.NStructure,
		NBureau:      report.NBureau,
		NCentrale:    report.NCentrale,
		NNonRattache: report.NNonRattache,
		NStructures:  report.NStructuresVues,
		ImportedBy:   p.ImportedBy,
		SourceFile:   p.SourceFile,
	}, eff, pyr, report.Inconnus)
	if err != nil {
		return nil, fmt.Errorf("enregistrement : %w", err)
	}
	log.Printf("[DRH] millésime %d enregistré (%d cellules d'effectif, %d de pyramide) en %s",
		im.ID, len(eff), len(pyr), time.Since(start).Round(time.Millisecond))

	return &DrhImportResult{Import: im, Report: report, DureeMs: time.Since(start).Milliseconds()}, nil
}
