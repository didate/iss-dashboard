package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/typologie"
)

// --- Snapshot blobs ---------------------------------------------------------

// SnapshotBlob is a pre-serialized payload with its cache validators.
type SnapshotBlob struct {
	Key     string
	ETag    string
	JSON    []byte
	BuiltAt string
}

// GetSnapshotBlob returns nil, nil when the blob has not been built yet (no sync).
func (s *Store) GetSnapshotBlob(key string) (*SnapshotBlob, error) {
	b := &SnapshotBlob{Key: key}
	err := s.db.QueryRow(`SELECT etag, json, built_at FROM snapshot_blob WHERE key=?`, key).Scan(&b.ETag, &b.JSON, &b.BuiltAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

// --- Public structures --------------------------------------------------------

// serviceOptionSet is the option set shared by every ISS service field.
const serviceOptionSet = "RGsTov6dBHH"

// PublicSearchParams filters the public structure search. Lat/Lng enable the
// "around me" mode: results are then sorted by distance and limited to RadiusKm.
type PublicSearchParams struct {
	Search         string
	Type           string
	Service        string // short key, e.g. MATERNITE
	District       string
	Region         string
	SousPrefecture string
	Lat, Lng       *float64
	RadiusKm       float64
	Limit          int
}

// publicWhere builds the WHERE clause shared by the public search, the annuaire
// and its CSV export. SECURITY: only hardcoded conditions go in where[]; user
// values go in args[] as placeholders.
func publicWhere(p PublicSearchParams) ([]string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if p.Search != "" {
		where = append(where, "e.org_unit_name LIKE ?")
		args = append(args, "%"+p.Search+"%")
	}
	if p.Type != "" {
		where = append(where, "e.type_code = ?")
		args = append(args, p.Type)
	}
	if p.District != "" {
		where = append(where, "e.district = ?")
		args = append(args, p.District)
	}
	if p.Region != "" {
		where = append(where, "e.region = ?")
		args = append(args, p.Region)
	}
	if p.SousPrefecture != "" {
		where = append(where, "e.sous_prefecture = ?")
		args = append(args, p.SousPrefecture)
	}
	if p.Service != "" {
		where = append(where, "EXISTS (SELECT 1 FROM event_value ev WHERE ev.event_uid = e.event_uid AND ev.de_code = ? AND ev.value = 'oui')")
		args = append(args, "ISS_SVC_"+strings.ToUpper(p.Service)+"_DE")
	}
	return where, args
}

func (s *Store) SearchPublicStructures(p PublicSearchParams) ([]models.PublicStructureItem, error) {
	if p.Limit < 1 || p.Limit > 200 {
		p.Limit = 50
	}
	near := p.Lat != nil && p.Lng != nil

	where, args := publicWhere(p)
	if near {
		where = append(where, "e.lat IS NOT NULL AND e.lng IS NOT NULL")
	}

	query := fmt.Sprintf(`
		SELECT e.org_unit_uid, e.org_unit_name, e.type_code, e.region, e.district, e.lat, e.lng,
		       COALESCE((SELECT ev.value FROM event_value ev WHERE ev.event_uid = e.event_uid AND ev.de_code = 'ISS_STATUT_OP_DE'), '')
		FROM structure_latest e
		WHERE %s
		ORDER BY e.org_unit_name`, strings.Join(where, " AND "))
	if !near {
		query += " LIMIT ?"
		args = append(args, p.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.PublicStructureItem
	for rows.Next() {
		var it models.PublicStructureItem
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&it.UID, &it.Name, &it.TypeCode, &it.Region, &it.District, &lat, &lng, &it.StatutOp); err != nil {
			return nil, err
		}
		it.TypeLabel = typologie.Label(it.TypeCode)
		if lat.Valid && lng.Valid {
			it.Lat, it.Lng = &lat.Float64, &lng.Float64
		}
		if near {
			d := haversineKm(*p.Lat, *p.Lng, lat.Float64, lng.Float64)
			if p.RadiusKm > 0 && d > p.RadiusKm {
				continue
			}
			it.DistanceKm = &d
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if near {
		sort.Slice(items, func(i, j int) bool { return *items[i].DistanceKm < *items[j].DistanceKm })
		if len(items) > p.Limit {
			items = items[:p.Limit]
		}
	}
	if items == nil {
		items = []models.PublicStructureItem{}
	}
	return items, nil
}

// PublicAnnuaireRow is one line of the public registry (annuaire) and its CSV export.
type PublicAnnuaireRow struct {
	UID            string   `json:"uid"`
	Name           string   `json:"name"`
	TypeCode       string   `json:"type"`
	TypeLabel      string   `json:"type_label"`
	StatutJuri     string   `json:"statut"`
	StatutOp       string   `json:"op"`
	Region         string   `json:"region"`
	District       string   `json:"district"`
	SousPrefecture string   `json:"sous_prefecture"`
	Lat            *float64 `json:"lat"`
	Lng            *float64 `json:"lng"`
	NServices      int      `json:"n_services"`
}

type PublicAnnuaireResult struct {
	Data     []PublicAnnuaireRow `json:"data"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// ListPublicStructures is the paginated registry. PageSize 0 disables pagination (CSV export).
func (s *Store) ListPublicStructures(p PublicSearchParams, page, pageSize int) (*PublicAnnuaireResult, error) {
	if page < 1 {
		page = 1
	}
	where, args := publicWhere(p)
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM structure_latest e WHERE `+whereClause, args...).Scan(&total); err != nil {
		return nil, err
	}

	query := `
		SELECT e.org_unit_uid, e.org_unit_name, COALESCE(e.type_code,''), e.region, e.district, COALESCE(e.sous_prefecture,''), e.lat, e.lng,
		       COALESCE((SELECT ev.value FROM event_value ev WHERE ev.event_uid = e.event_uid AND ev.de_code = 'ISS_STATUT_STRUCT_DE'), ''),
		       COALESCE((SELECT ev.value FROM event_value ev WHERE ev.event_uid = e.event_uid AND ev.de_code = 'ISS_STATUT_OP_DE'), ''),
		       (SELECT COUNT(*) FROM event_value ev JOIN metadata_de md ON md.de_uid = ev.de_uid
		         WHERE ev.event_uid = e.event_uid AND ev.value = 'oui' AND md.option_set_id = ? AND md.section_prefix IN ('ISS_SVC','ISS_LAB'))
		FROM structure_latest e WHERE ` + whereClause + ` ORDER BY e.region, e.district, e.org_unit_name`
	args = append([]any{serviceOptionSet}, args...)
	if pageSize > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, pageSize, (page-1)*pageSize)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &PublicAnnuaireResult{Data: []PublicAnnuaireRow{}, Total: total, Page: page, PageSize: pageSize}
	for rows.Next() {
		var r PublicAnnuaireRow
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&r.UID, &r.Name, &r.TypeCode, &r.Region, &r.District, &r.SousPrefecture, &lat, &lng, &r.StatutJuri, &r.StatutOp, &r.NServices); err != nil {
			return nil, err
		}
		r.TypeLabel = typologie.Label(r.TypeCode)
		if lat.Valid && lng.Valid {
			r.Lat, r.Lng = &lat.Float64, &lng.Float64
		}
		res.Data = append(res.Data, r)
	}
	return res, rows.Err()
}

// haversineKm is the great-circle distance between two WGS84 points.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// plateauServices maps the public "plateau technique" keys to service DE codes.
var plateauServices = map[string]string{
	"labo":      "ISS_SVC_LABO_DE",
	"maternite": "ISS_SVC_MATERNITE_DE",
	"imagerie":  "ISS_SVC_IMAGERIE_DE",
	"urgences":  "ISS_SVC_URGENCES_DE",
	"pharmacie": "ISS_SVC_PHARMACIE_DE",
	"chirurgie": "ISS_SVC_CHIRURGIE_DE",
}

// GetPublicStructure builds the reduced public record of a structure from its
// most recent event. Returns nil, nil when the org unit is unknown. Only the
// fields listed here ever leave the server: adding one is a product decision.
func (s *Store) GetPublicStructure(ouUID string) (*models.PublicStructure, error) {
	ps := &models.PublicStructure{Plateau: map[string]bool{}, Services: []models.PublicService{}}
	var eventUID string
	var lat, lng sql.NullFloat64
	err := s.db.QueryRow(`SELECT event_uid, org_unit_uid, org_unit_name, type_code, region, district, sous_prefecture, lat, lng, event_date
		FROM structure_latest WHERE org_unit_uid = ?`, ouUID).
		Scan(&eventUID, &ps.UID, &ps.Name, &ps.TypeCode, &ps.Region, &ps.District, &ps.SousPrefecture, &lat, &lng, &ps.RecenseLe)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ps.TypeLabel = typologie.Label(ps.TypeCode)
	if lat.Valid && lng.Valid {
		ps.Lat, ps.Lng = &lat.Float64, &lng.Float64
	}
	if len(ps.RecenseLe) > 10 {
		ps.RecenseLe = ps.RecenseLe[:10]
	}

	// Statuts (option codes) and services, in one pass over the event's values.
	rows, err := s.db.Query(`
		SELECT ev.de_code, ev.value, COALESCE(NULLIF(md.form_name,''), md.name, ''), COALESCE(md.section_prefix,''), COALESCE(md.option_set_id,'')
		FROM event_value ev LEFT JOIN metadata_de md ON md.de_uid = ev.de_uid
		WHERE ev.event_uid = ?`, eventUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code, value, name, section, optionSet string
		if err := rows.Scan(&code, &value, &name, &section, &optionSet); err != nil {
			return nil, err
		}
		switch code {
		case "ISS_STATUT_STRUCT_DE":
			ps.StatutJuri = value
		case "ISS_STATUT_PUB_DE", "ISS_STATUT_PRIV_DE":
			if value != "" {
				ps.StatutDetail = value
			}
		case "ISS_STATUT_OP_DE":
			ps.StatutOp = value
		}
		if (section == "ISS_SVC" || section == "ISS_LAB") && optionSet == serviceOptionSet && value == "oui" {
			key := strings.TrimSuffix(strings.TrimPrefix(code, "ISS_SVC_"), "_DE")
			label := strings.TrimPrefix(strings.TrimPrefix(name, "ISS_SVC "), "ISS_LAB ")
			ps.Services = append(ps.Services, models.PublicService{Code: key, Label: label})
		}
		for pk, pcode := range plateauServices {
			if code == pcode {
				ps.Plateau[pk] = value == "oui"
			}
		}
	}
	sort.Slice(ps.Services, func(i, j int) bool { return ps.Services[i].Label < ps.Services[j].Label })
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Agrégats calculés au sync (mêmes chiffres que le popup de la carte)
	var rhT, rhM, rhS, eau, en, sc sql.NullInt64
	err = s.db.QueryRow(`SELECT niveau, rh_total, rh_medecins, rh_soignants, eau, energie, score_services, score_services_max FROM public_extra WHERE ou_uid = ?`, ouUID).
		Scan(&ps.Niveau, &rhT, &rhM, &rhS, &eau, &en, &sc, &ps.ScoreServicesN)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	toInt := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		i := int(v.Int64)
		return &i
	}
	toBool := func(v sql.NullInt64) *bool {
		if !v.Valid {
			return nil
		}
		b := v.Int64 == 1
		return &b
	}
	ps.RhTotal, ps.RhMedecins, ps.RhSoignants = toInt(rhT), toInt(rhM), toInt(rhS)
	ps.Eau, ps.Energie, ps.ScoreServices = toBool(eau), toBool(en), toInt(sc)
	return ps, nil
}

func (s *Store) GetPublicSummary() (*models.PublicSummary, error) {
	out := &models.PublicSummary{NParType: map[string]int{}}
	var nGPS int
	if err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(lat IS NOT NULL),0) FROM structure_latest`).Scan(&out.NStructures, &nGPS); err != nil {
		return nil, err
	}
	if out.NStructures > 0 {
		out.PctGPS = math.Round(1000*float64(nGPS)/float64(out.NStructures)) / 10
	}
	rows, err := s.db.Query(`SELECT COALESCE(NULLIF(type_code,''), 'INDETERMINE'), COUNT(*) FROM structure_latest GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out.NParType[code] = n
	}
	if sr, _ := s.GetLastSyncRun(); sr != nil && sr.FinishedAt != nil {
		out.DerniereSynchro = sr.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return out, rows.Err()
}

// --- Pro : couverture géographique ------------------------------------------

// GetGeoCoverage returns usage_geo rows for one level, optionally restricted to a region.
func (s *Store) GetGeoCoverage(level int, region string) ([]models.UsageGeo, error) {
	if level != 3 && level != 4 {
		level = 3
	}
	query := `SELECT g.level, g.ou_uid, g.name, g.parent_name, g.n_structures, g.n_gps, g.pct_gps, g.avg_score, g.population, g.n_par_type
		FROM usage_geo g`
	args := []any{}
	switch {
	case region != "" && level == 3:
		query += ` JOIN org_unit d ON d.uid = g.ou_uid WHERE g.level = ? AND d.parent_name = ?`
		args = append(args, level, region)
	case region != "" && level == 4:
		query += ` JOIN org_unit sp ON sp.uid = g.ou_uid JOIN org_unit d ON d.uid = sp.parent_uid WHERE g.level = ? AND d.parent_name = ?`
		args = append(args, level, region)
	default:
		query += ` WHERE g.level = ?`
		args = append(args, level)
	}
	query += ` ORDER BY g.parent_name, g.name`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.UsageGeo{}
	for rows.Next() {
		g, err := scanUsageGeo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func scanUsageGeo(rows *sql.Rows) (models.UsageGeo, error) {
	var g models.UsageGeo
	var pctGPS, avgScore, population sql.NullFloat64
	var parType string
	if err := rows.Scan(&g.Level, &g.OrgUnitUID, &g.Name, &g.ParentName, &g.NStructures, &g.NGPS, &pctGPS, &avgScore, &population, &parType); err != nil {
		return g, err
	}
	if pctGPS.Valid {
		g.PctGPS = &pctGPS.Float64
	}
	if avgScore.Valid {
		g.AvgScore = &avgScore.Float64
	}
	if population.Valid {
		g.Population = &population.Float64
	}
	g.NParType = map[string]int{}
	json.Unmarshal([]byte(parType), &g.NParType)
	return g, nil
}

// MissingGPSParams filters the "structures without coordinates" list.
type MissingGPSParams struct {
	District string
	Region   string
	Type     string
	Page     int
	PageSize int
}

type MissingGPSResult struct {
	Data     []models.MissingGPSItem `json:"data"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// GetMissingGPS lists structures (latest event per org unit) without a point.
// PageSize 0 disables pagination (CSV export).
func (s *Store) GetMissingGPS(p MissingGPSParams) (*MissingGPSResult, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	where := []string{"e.lat IS NULL"}
	args := []any{}
	if p.District != "" {
		where = append(where, "e.district = ?")
		args = append(args, p.District)
	}
	if p.Region != "" {
		where = append(where, "e.region = ?")
		args = append(args, p.Region)
	}
	if p.Type != "" {
		where = append(where, "e.type_code = ?")
		args = append(args, p.Type)
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM structure_latest e WHERE `+whereClause, args...).Scan(&total); err != nil {
		return nil, err
	}

	query := `SELECT e.event_uid, e.org_unit_uid, e.org_unit_name, e.type_code, e.region, e.district, e.sous_prefecture, e.event_date
		FROM structure_latest e WHERE ` + whereClause + ` ORDER BY e.region, e.district, e.sous_prefecture, e.org_unit_name`
	if p.PageSize > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, p.PageSize, (p.Page-1)*p.PageSize)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &MissingGPSResult{Data: []models.MissingGPSItem{}, Total: total, Page: p.Page, PageSize: p.PageSize}
	for rows.Next() {
		var it models.MissingGPSItem
		if err := rows.Scan(&it.EventUID, &it.OrgUnitUID, &it.Name, &it.TypeCode, &it.Region, &it.District, &it.SousPrefecture, &it.EventDate); err != nil {
			return nil, err
		}
		it.TypeLabel = typologie.Label(it.TypeCode)
		res.Data = append(res.Data, it)
	}
	return res, rows.Err()
}

// GetCouverture returns demographic ratios for one dimension, optionally one indicator.
func (s *Store) GetCouverture(dimension, indicator string) ([]models.UsageCouverture, error) {
	switch dimension {
	case "global", "region", "district", "sous_prefecture":
	default:
		dimension = "district"
	}
	query := `SELECT dimension, key, COALESCE(ou_uid,''), label, indicator, numerator, population, ratio_10k FROM usage_couverture WHERE dimension = ?`
	args := []any{dimension}
	if indicator != "" {
		query += ` AND indicator = ?`
		args = append(args, indicator)
	}
	query += ` ORDER BY key, indicator`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.UsageCouverture{}
	for rows.Next() {
		var c models.UsageCouverture
		var pop, ratio sql.NullFloat64
		if err := rows.Scan(&c.Dimension, &c.Key, &c.OrgUnitUID, &c.Label, &c.Indicator, &c.Numerator, &pop, &ratio); err != nil {
			return nil, err
		}
		if pop.Valid {
			c.Population = &pop.Float64
		}
		if ratio.Valid {
			c.Ratio10k = &ratio.Float64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MapGeoProperties are the choropleth properties of one administrative unit (level 3 or 4).
type MapGeoProperties struct {
	models.UsageGeo
	RatioStructures10k *float64 `json:"ratio_structures_10k"`
	// Ratios pour 10 000 hab. par indicateur de couverture (lits, medecins, sages_femmes, …)
	Ratios     map[string]*float64 `json:"ratios"`
	Numerators map[string]float64  `json:"numerators"`
	// Conformité aux normes (nil sans référentiel actif)
	ConformiteScore *float64 `json:"conformite_score"`
	PctConformes    *float64 `json:"pct_conformes"`
}

type MapGeoFeature struct {
	Type       string           `json:"type"`
	Geometry   json.RawMessage  `json:"geometry"`
	Properties MapGeoProperties `json:"properties"`
}

type MapGeoCollection struct {
	Type     string          `json:"type"`
	Features []MapGeoFeature `json:"features"`
}

// GetMapGeo returns the polygons of one level with their usage_geo properties.
// Units without geometry are skipped (they cannot be drawn) but stay in GetGeoCoverage.
func (s *Store) GetMapGeo(level int) (*MapGeoCollection, error) {
	units, err := s.GetGeoCoverage(level, "")
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT uid, geometry FROM org_unit WHERE level = ? AND geometry != ''`, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	geoms := map[string]string{}
	for rows.Next() {
		var uid, geom string
		if err := rows.Scan(&uid, &geom); err != nil {
			return nil, err
		}
		geoms[uid] = geom
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Couverture démographique par unité (une ligne par indicateur).
	cRows, err := s.db.Query(`SELECT ou_uid, indicator, numerator, ratio_10k FROM usage_couverture WHERE ou_uid != '' AND dimension = ?`,
		map[int]string{3: "district", 4: "sous_prefecture"}[level])
	if err != nil {
		return nil, err
	}
	defer cRows.Close()
	ratios := map[string]map[string]*float64{}
	numerators := map[string]map[string]float64{}
	for cRows.Next() {
		var uid, ind string
		var num float64
		var ratio sql.NullFloat64
		if err := cRows.Scan(&uid, &ind, &num, &ratio); err != nil {
			return nil, err
		}
		if ratios[uid] == nil {
			ratios[uid] = map[string]*float64{}
			numerators[uid] = map[string]float64{}
		}
		numerators[uid][ind] = num
		if ratio.Valid {
			r := ratio.Float64
			ratios[uid][ind] = &r
		} else {
			ratios[uid][ind] = nil
		}
	}
	if err := cRows.Err(); err != nil {
		return nil, err
	}

	conformite, err := s.ConformiteByOrgUnit(level)
	if err != nil {
		return nil, err
	}

	fc := &MapGeoCollection{Type: "FeatureCollection", Features: []MapGeoFeature{}}
	for _, g := range units {
		geom, ok := geoms[g.OrgUnitUID]
		if !ok {
			continue
		}
		props := MapGeoProperties{UsageGeo: g, Ratios: ratios[g.OrgUnitUID], Numerators: numerators[g.OrgUnitUID]}
		if cf, ok := conformite[g.Name]; ok {
			props.ConformiteScore, props.PctConformes = cf.AvgScore, cf.PctConforme
		}
		if props.Ratios == nil {
			props.Ratios = map[string]*float64{}
			props.Numerators = map[string]float64{}
		}
		if g.Population != nil && *g.Population > 0 {
			r := float64(g.NStructures) / *g.Population * 10000
			props.RatioStructures10k = &r
		}
		fc.Features = append(fc.Features, MapGeoFeature{Type: "Feature", Geometry: json.RawMessage(geom), Properties: props})
	}
	return fc, nil
}

// --- Pro : points des structures avec qualité ---------------------------------

// ProPointProperties are the planners' map point properties (quality included).
type ProPointProperties struct {
	EventUID      string `json:"event_uid"`
	UID           string `json:"uid"`
	Name          string `json:"name"`
	TypeCode      string `json:"type"`
	TypeLabel     string `json:"type_label"`
	Region        string `json:"region"`
	District      string `json:"district"`
	Score         int    `json:"score"`
	WorstSeverity string `json:"worst_severity"`
	NIssues       int    `json:"n_issues"`
}

type ProPointFeature struct {
	Type       string             `json:"type"`
	Geometry   json.RawMessage    `json:"geometry"`
	Properties ProPointProperties `json:"properties"`
}

type ProPointCollection struct {
	Type     string            `json:"type"`
	Features []ProPointFeature `json:"features"`
}

// GetProPoints returns every geolocated structure (latest event) with its quality summary.
func (s *Store) GetProPoints() (*ProPointCollection, error) {
	rows, err := s.db.Query(`
		SELECT e.event_uid, e.org_unit_uid, e.org_unit_name, COALESCE(e.type_code,''), e.region, e.district, e.lat, e.lng,
		       COALESCE(q.score, 100), COALESCE(q.worst_severity,''), COALESCE(q.n_error,0)+COALESCE(q.n_warning,0)+COALESCE(q.n_info,0)
		FROM structure_latest e LEFT JOIN event_quality q ON q.event_uid = e.event_uid
		WHERE e.lat IS NOT NULL AND e.lng IS NOT NULL
		ORDER BY e.org_unit_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fc := &ProPointCollection{Type: "FeatureCollection", Features: []ProPointFeature{}}
	for rows.Next() {
		var p ProPointProperties
		var lat, lng float64
		if err := rows.Scan(&p.EventUID, &p.UID, &p.Name, &p.TypeCode, &p.Region, &p.District, &lat, &lng, &p.Score, &p.WorstSeverity, &p.NIssues); err != nil {
			return nil, err
		}
		p.TypeLabel = typologie.Label(p.TypeCode)
		geom, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{lng, lat}})
		fc.Features = append(fc.Features, ProPointFeature{Type: "Feature", Geometry: geom, Properties: p})
	}
	return fc, rows.Err()
}

// --- Agrégats régionaux (somme des districts, pour le rapport PDF par région) ----

// districtsOfRegionSQL is a subquery of district names (level 3) belonging to a region name.
const districtsOfRegionSQL = `SELECT d.name FROM org_unit d JOIN org_unit r ON r.uid = d.parent_uid WHERE d.level = 3 AND r.name = ?`

func (s *Store) GetUsageServicesRegion(region string) ([]models.UsageService, error) {
	rows, err := s.db.Query(`
		SELECT service_code, MAX(service_label), SUM(n_oui), SUM(n_oui_pas_fonc), SUM(n_non), SUM(n_total)
		FROM usage_service WHERE district IN (`+districtsOfRegionSQL+`)
		GROUP BY service_code ORDER BY MAX(service_label)`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageService
	for rows.Next() {
		var r models.UsageService
		if err := rows.Scan(&r.ServiceCode, &r.ServiceLabel, &r.NOui, &r.NOuiPasFonc, &r.NNon, &r.NTotal); err != nil {
			return nil, err
		}
		r.District = region
		if r.NTotal > 0 {
			r.PctFonctionnel = 100 * float64(r.NOui) / float64(r.NTotal)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PctFonctionnel > out[j].PctFonctionnel })
	return out, rows.Err()
}

func (s *Store) GetUsageEquipementsRegion(region string) ([]models.UsageEquipement, error) {
	rows, err := s.db.Query(`
		SELECT equip_root, MAX(label), MAX(category), SUM(sum_total), SUM(sum_fonct)
		FROM usage_equipement WHERE district IN (`+districtsOfRegionSQL+`)
		GROUP BY equip_root ORDER BY MAX(label)`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageEquipement
	for rows.Next() {
		var r models.UsageEquipement
		if err := rows.Scan(&r.EquipRoot, &r.Label, &r.Category, &r.SumTotal, &r.SumFonct); err != nil {
			return nil, err
		}
		r.District = region
		if r.SumTotal > 0 {
			r.PctFonct = 100 * float64(r.SumFonct) / float64(r.SumTotal)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetUsageCommoditesRegion(region string) ([]models.UsageCommodite, error) {
	rows, err := s.db.Query(`
		SELECT indicator, SUM(n_oui), SUM(n_total)
		FROM usage_commodite WHERE district IN (`+districtsOfRegionSQL+`)
		GROUP BY indicator ORDER BY indicator`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageCommodite
	for rows.Next() {
		var r models.UsageCommodite
		if err := rows.Scan(&r.Indicator, &r.NOui, &r.NTotal); err != nil {
			return nil, err
		}
		r.District = region
		if r.NTotal > 0 {
			r.Pct = 100 * float64(r.NOui) / float64(r.NTotal)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRHSummaryRegion sums the RH figures of a region's districts (ratio per structure
// and structures without doctor computed on the region's events).
func (s *Store) GetRHSummaryRegion(region string) (*RHSummaryResult, error) {
	r := &RHSummaryResult{}
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(effectif_fonc),0), COALESCE(SUM(effectif_contr),0), COALESCE(SUM(effectif_benev),0), COALESCE(SUM(effectif_asc),0), COALESCE(SUM(effectif_reco),0), COALESCE(SUM(effectif_total),0)
		FROM usage_rh WHERE district IN (`+districtsOfRegionSQL+`)`, region).
		Scan(&r.TotalFonc, &r.TotalContr, &r.TotalBenev, &r.TotalASC, &r.TotalRECO, &r.TotalEffectif); err != nil {
		return nil, err
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM event WHERE region = ?`, region).Scan(&r.NStructures)
	var totalMed int
	s.db.QueryRow(`SELECT COALESCE(SUM(effectif_total),0) FROM usage_rh WHERE district IN (`+districtsOfRegionSQL+`) AND profil_code LIKE 'ISS_RH_MED_%'`, region).Scan(&totalMed)
	if r.NStructures > 0 {
		r.RatioMedPerStr = float64(totalMed) / float64(r.NStructures)
	}
	s.db.QueryRow(`
		SELECT COUNT(*) FROM event e WHERE e.region = ? AND e.event_uid NOT IN (
			SELECT ev.event_uid FROM event_value ev WHERE ev.de_code LIKE 'ISS_RH_MED_%' AND CAST(ev.value AS REAL) > 0)`, region).Scan(&r.NStrSansMed)
	if r.NStructures > 0 {
		r.PctStrSansMed = 100 * float64(r.NStrSansMed) / float64(r.NStructures)
	}
	return r, nil
}
