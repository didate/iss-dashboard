package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	baseURL   = os.Getenv("DHIS2_BASE_URL")
	pat       = os.Getenv("DHIS2_PAT")
	programID = "AJy1cnAA50U"
	dryRun    = false
)

type DataElement struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type DataValue struct {
	DataElement string `json:"dataElement"`
	Value       string `json:"value"`
}

type Event struct {
	Event       string      `json:"event"`
	OrgUnit     string      `json:"orgUnit"`
	OrgUnitName string      `json:"orgUnitName"`
	DataValues  []DataValue `json:"dataValues"`
}

type EquipPair struct {
	Root      string
	TotalUID  string
	TotalName string
	FoncUID   string
	FoncName  string
}

type Fix struct {
	EventUID     string
	OrgUnit      string
	TotalUID     string
	TotalName    string
	FoncName     string
	FoncValue    string
	TotalCurrent string
}

func doGet(path string) ([]byte, error) {
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.Header.Set("Authorization", "ApiToken "+pat)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return body, nil
}

func getEquipmentPairs() ([]EquipPair, error) {
	log.Println("[1/4] Récupération des data elements...")
	body, err := doGet("/api/dataElements.json?filter=name:like:ISS&fields=id,code,name&paging=false")
	if err != nil {
		return nil, err
	}
	var resp struct {
		DataElements []DataElement `json:"dataElements"`
	}
	json.Unmarshal(body, &resp)

	totals := map[string]DataElement{}
	foncs := map[string]DataElement{}
	for _, de := range resp.DataElements {
		if strings.HasSuffix(de.Code, "_TOTAL_DE") {
			totals[strings.TrimSuffix(de.Code, "_TOTAL_DE")] = de
		} else if strings.HasSuffix(de.Code, "_FONC_DE") {
			foncs[strings.TrimSuffix(de.Code, "_FONC_DE")] = de
		}
	}

	var pairs []EquipPair
	for root, f := range foncs {
		if t, ok := totals[root]; ok {
			pairs = append(pairs, EquipPair{
				Root: root, TotalUID: t.ID, TotalName: t.Name, FoncUID: f.ID, FoncName: f.Name,
			})
		}
	}

	// Paires manuelles pour les DE avec codes irréguliers
	deByUID := map[string]DataElement{}
	for _, de := range resp.DataElements {
		deByUID[de.ID] = de
	}
	manualPairs := [][2]string{
		{"dbvbR4uv1Vj", "Es7t9jdPlT5"}, // Table pansement: ISS_EQUI_TABLE_PAN_TOTAL_DE ↔ ISS_EQUI_TABLE__PAN_OP_FONC_DE
		{"I50d9n8jpCk", "iGQWP2HFv5Y"}, // Pèse-bébés: ISS_NB_PESE_BEBE ↔ ISS_NB_PESE_BEBE_FONC_DE
	}
	for _, mp := range manualPairs {
		t, okT := deByUID[mp[0]]
		f, okF := deByUID[mp[1]]
		if okT && okF {
			pairs = append(pairs, EquipPair{
				Root: "MANUAL_" + t.Code, TotalUID: t.ID, TotalName: t.Name, FoncUID: f.ID, FoncName: f.Name,
			})
		}
	}
	log.Printf("   %d paires total/fonctionnel trouvées", len(pairs))
	return pairs, nil
}

func fetchAllEvents() ([]Event, error) {
	log.Println("[2/4] Récupération des events...")
	var all []Event
	page := 1

	for {
		path := fmt.Sprintf("/api/events.json?program=%s&fields=event,orgUnit,orgUnitName,dataValues[dataElement,value]&pageSize=200&page=%d&totalPages=true", programID, page)
		body, err := doGet(path)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Events []Event `json:"events"`
			Pager  struct {
				PageCount int `json:"pageCount"`
			} `json:"pager"`
		}
		json.Unmarshal(body, &resp)
		all = append(all, resp.Events...)
		log.Printf("   Page %d/%d — %d events (total: %d)", page, resp.Pager.PageCount, len(resp.Events), len(all))
		if page >= resp.Pager.PageCount {
			break
		}
		page++
	}
	return all, nil
}

// Règles de correction de valeurs invalides d'optionSet
var optionFixRules = []struct {
	DEUID       string
	BadValue    string
	GoodValue   string
	Description string
}{
	{"IzfXJ0Zrfxh", "forage", "FEM", "Source d'eau: forage → FEM"},
}

// Règles pour les valeurs manquantes à remplir avec une valeur par défaut
var missingValueRules = []struct {
	DEUID        string
	DefaultValue string
	Description  string
}{
	{"HpjvSNCEWM0", "operationnel", "Statut opérationnel manquant → operationnel"},
}

func findCases(events []Event, pairs []EquipPair) []Fix {
	log.Println("[3/4] Analyse des incohérences...")
	foncToPair := map[string]EquipPair{}
	for _, p := range pairs {
		foncToPair[p.FoncUID] = p
	}

	var fixes []Fix
	for _, evt := range events {
		vals := map[string]string{}
		for _, dv := range evt.DataValues {
			vals[dv.DataElement] = dv.Value
		}

		// R2a: Total vide/0 alors que Fonctionnel > 0 → total = fonctionnel
		// R2b: Fonctionnel > Total → total = fonctionnel + total
		for foncUID, pair := range foncToPair {
			foncVal := toInt(vals[foncUID])
			totalVal := toInt(vals[pair.TotalUID])
			if foncVal > 0 && totalVal <= 0 {
				// Total absent/0 → copier fonctionnel
				fixes = append(fixes, Fix{
					EventUID:     evt.Event,
					OrgUnit:      evt.OrgUnitName,
					TotalUID:     pair.TotalUID,
					TotalName:    pair.TotalName,
					FoncName:     pair.FoncName,
					FoncValue:    vals[foncUID],
					TotalCurrent: valOrAbsent(vals[pair.TotalUID]),
				})
			} else if foncVal > 0 && foncVal > totalVal {
				// Fonctionnel > Total → total = fonctionnel + total
				newTotal := strconv.Itoa(foncVal + totalVal)
				fixes = append(fixes, Fix{
					EventUID:     evt.Event,
					OrgUnit:      evt.OrgUnitName,
					TotalUID:     pair.TotalUID,
					TotalName:    pair.TotalName,
					FoncName:     pair.FoncName,
					FoncValue:    newTotal,
					TotalCurrent: vals[pair.TotalUID],
				})
			}
		}

		// Valeurs invalides d'optionSet
		for _, rule := range optionFixRules {
			if val, ok := vals[rule.DEUID]; ok && val == rule.BadValue {
				fixes = append(fixes, Fix{
					EventUID:     evt.Event,
					OrgUnit:      evt.OrgUnitName,
					TotalUID:     rule.DEUID,
					TotalName:    rule.Description,
					FoncName:     "R9",
					FoncValue:    rule.GoodValue,
					TotalCurrent: rule.BadValue,
				})
			}
		}

		// Valeurs manquantes à remplir
		for _, rule := range missingValueRules {
			if val := vals[rule.DEUID]; val == "" {
				fixes = append(fixes, Fix{
					EventUID:     evt.Event,
					OrgUnit:      evt.OrgUnitName,
					TotalUID:     rule.DEUID,
					TotalName:    rule.Description,
					FoncName:     "default",
					FoncValue:    rule.DefaultValue,
					TotalCurrent: "(absent)",
				})
			}
		}
	}
	log.Printf("   %d cas trouvés", len(fixes))
	return fixes
}

func applyFixes(fixes []Fix) (int, int) {
	// Grouper par event
	grouped := map[string][]Fix{}
	var order []string
	for _, f := range fixes {
		if _, exists := grouped[f.EventUID]; !exists {
			order = append(order, f.EventUID)
		}
		grouped[f.EventUID] = append(grouped[f.EventUID], f)
	}

	mode := "EXÉCUTION RÉELLE"
	if dryRun {
		mode = "DRY RUN"
	}
	log.Printf("[4/4] %s — %d corrections sur %d events\n", mode, len(fixes), len(grouped))

	fixed, errors := 0, 0

	for i, eventUID := range order {
		cases := grouped[eventUID]
		orgUnit := cases[0].OrgUnit

		if dryRun {
			for _, c := range cases {
				fmt.Printf("  [%d/%d] %s — %s: %s → %s\n", i+1, len(order), orgUnit, c.TotalName, c.TotalCurrent, c.FoncValue)
			}
			fixed += len(cases)
			continue
		}

		err := updateEvent(eventUID, cases)
		if err != nil {
			fmt.Printf("  ✗ [%d/%d] %s — %v\n", i+1, len(order), orgUnit, err)
			errors += len(cases)
			continue
		}
		for _, c := range cases {
			fmt.Printf("  ✓ [%d/%d] %s — %s: %s → %s\n", i+1, len(order), orgUnit, c.TotalName, c.TotalCurrent, c.FoncValue)
		}
		fixed += len(cases)
	}
	return fixed, errors
}

func updateEvent(eventUID string, cases []Fix) error {
	// GET l'event complet
	body, err := doGet("/api/events/" + eventUID + ".json")
	if err != nil {
		return fmt.Errorf("GET: %w", err)
	}

	var eventData map[string]interface{}
	json.Unmarshal(body, &eventData)

	// Supprimer les notes pour éviter les conflits de clé unique
	delete(eventData, "notes")

	// Modifier les dataValues
	dvRaw, _ := eventData["dataValues"].([]interface{})
	if dvRaw == nil {
		dvRaw = []interface{}{}
	}
	dvMap := map[string]int{} // dataElement -> index
	for i, raw := range dvRaw {
		dv, _ := raw.(map[string]interface{})
		de, _ := dv["dataElement"].(string)
		dvMap[de] = i
	}

	for _, c := range cases {
		if idx, exists := dvMap[c.TotalUID]; exists {
			dv := dvRaw[idx].(map[string]interface{})
			dv["value"] = c.FoncValue
		} else {
			dvRaw = append(dvRaw, map[string]interface{}{
				"dataElement": c.TotalUID,
				"value":       c.FoncValue,
			})
		}
	}
	eventData["dataValues"] = dvRaw

	// PUT
	payload, _ := json.Marshal(eventData)
	req, _ := http.NewRequest("PUT", baseURL+"/api/events/"+eventUID, bytes.NewReader(payload))
	req.Header.Set("Authorization", "ApiToken "+pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 150)]))
	}
	return nil
}

func toInt(s string) int {
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func valOrAbsent(s string) string {
	if s == "" {
		return "(absent)"
	}
	return s
}

func main() {
	if baseURL == "" || pat == "" {
		log.Fatal("DHIS2_BASE_URL et DHIS2_PAT doivent être définis")
	}

	for _, arg := range os.Args[1:] {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	http.DefaultClient.Timeout = 30 * time.Second

	fmt.Println("============================================================")
	fmt.Println("Correction ISS : Total = Fonctionnel quand Total est vide")
	if dryRun {
		fmt.Println("Mode : DRY RUN")
	} else {
		fmt.Println("Mode : EXÉCUTION RÉELLE")
	}
	fmt.Printf("Serveur : %s\n", baseURL)
	fmt.Printf("Date : %s\n", time.Now().Format("2006-01-02 15:04"))
	fmt.Println("============================================================")
	fmt.Println()

	pairs, err := getEquipmentPairs()
	if err != nil {
		log.Fatalf("Erreur metadata: %v", err)
	}

	events, err := fetchAllEvents()
	if err != nil {
		log.Fatalf("Erreur events: %v", err)
	}

	fixes := findCases(events, pairs)
	if len(fixes) == 0 {
		fmt.Println("\n✅ Aucune incohérence trouvée.")
		return
	}

	fmt.Println()
	fixed, errors := applyFixes(fixes)
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Printf("Terminé : %d corrigés, %d erreurs\n", fixed, errors)
	if dryRun {
		fmt.Println("\n⚠️  Dry run. Relancez sans --dry-run pour appliquer.")
	}
}
