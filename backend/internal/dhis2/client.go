package dhis2

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"iss-dashboard-backend/internal/models"
)

type Client struct {
	baseURL   string
	pat       string
	programID string
	http      *http.Client
}

func NewClient(baseURL, pat, programID string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		pat:       pat,
		programID: programID,
		http: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (c *Client) doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "ApiToken "+c.pat)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, string(body[:min(len(body), 500)]))
	}
	return io.ReadAll(resp.Body)
}

// FetchAllEvents fetches all events for the program, paginated.
func (c *Client) FetchAllEvents() ([]models.Event, error) {
	var all []models.Event
	page := 1
	pageSize := 200

	for {
		url := fmt.Sprintf("%s/api/events.json?program=%s&fields=event,orgUnit,orgUnitName,eventDate,status,dataValues[dataElement,value]&pageSize=%d&page=%d&totalPages=true",
			c.baseURL, c.programID, pageSize, page)

		log.Printf("[DHIS2] Fetching events page %d...", page)
		body, err := c.doGet(url)
		if err != nil {
			return nil, fmt.Errorf("fetch events page %d: %w", page, err)
		}

		var resp models.DHIS2EventsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse events page %d: %w", page, err)
		}

		// Store raw JSON per event
		for i := range resp.Events {
			raw, _ := json.Marshal(resp.Events[i])
			resp.Events[i].RawJSON = string(raw)
		}

		all = append(all, resp.Events...)
		log.Printf("[DHIS2] Page %d/%d: got %d events (total so far: %d)", page, resp.Pager.PageCount, len(resp.Events), len(all))

		if page >= resp.Pager.PageCount {
			break
		}
		page++
	}
	// Deduplicate events (DHIS2 pagination can return overlapping results)
	seen := make(map[string]bool, len(all))
	deduped := make([]models.Event, 0, len(all))
	for _, evt := range all {
		if !seen[evt.EventUID] {
			seen[evt.EventUID] = true
			deduped = append(deduped, evt)
		}
	}
	if len(deduped) < len(all) {
		log.Printf("[DHIS2] Deduplicated: %d → %d events (%d doublons ignores)", len(all), len(deduped), len(all)-len(deduped))
	}
	return deduped, nil
}

// FetchDataElements fetches ISS data element metadata.
func (c *Client) FetchDataElements() ([]models.DataElementMeta, error) {
	url := fmt.Sprintf("%s/api/dataElements.json?filter=name:like:ISS&fields=id,name,formName,code,valueType,optionSet[id]&paging=false", c.baseURL)
	body, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	var resp models.DHIS2DataElementsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var out []models.DataElementMeta
	for _, de := range resp.DataElements {
		meta := models.DataElementMeta{
			UID:       de.ID,
			Code:      de.Code,
			Name:      de.Name,
			FormName:  de.FormName,
			ValueType: de.ValueType,
		}
		if de.OptionSet != nil {
			meta.OptionSetID = de.OptionSet.ID
		}
		meta.SectionPrefix = deriveSectionPrefix(de.Code, de.Name)
		out = append(out, meta)
	}
	return out, nil
}

// FetchOptionSets fetches all option sets with their options.
func (c *Client) FetchOptionSets() ([]models.OptionEntry, error) {
	url := fmt.Sprintf("%s/api/optionSets.json?fields=id,name,options[code,name]&paging=false", c.baseURL)
	body, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	var resp models.DHIS2OptionSetsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var out []models.OptionEntry
	for _, os := range resp.OptionSets {
		for _, opt := range os.Options {
			out = append(out, models.OptionEntry{
				OptionSetID: os.ID,
				Code:        opt.Code,
				Name:        opt.Name,
			})
		}
	}
	return out, nil
}

// FetchOrgUnits fetches organisation units with parent info.
func (c *Client) FetchOrgUnits() ([]models.OrgUnit, error) {
	url := fmt.Sprintf("%s/api/organisationUnits.json?fields=id,name,level,parent[id,name],closedDate,geometry&paging=false", c.baseURL)
	body, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	var resp models.DHIS2OrgUnitsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var out []models.OrgUnit
	for _, ou := range resp.OrganisationUnits {
		o := models.OrgUnit{
			UID:        ou.ID,
			Name:       ou.Name,
			Level:      ou.Level,
			ClosedDate: ou.ClosedDate,
		}
		if ou.Parent != nil {
			o.ParentUID = ou.Parent.ID
			o.ParentName = ou.Parent.Name
		}
		if len(ou.Geometry) > 0 && string(ou.Geometry) != "null" {
			o.Geometry = string(ou.Geometry)
		}
		out = append(out, o)
	}
	return out, nil
}

// FetchProgramOrgUnits returns the UIDs of org units assigned to the program.
func (c *Client) FetchProgramOrgUnits() ([]string, error) {
	url := fmt.Sprintf("%s/api/programs/%s.json?fields=organisationUnits[id]", c.baseURL, c.programID)
	body, err := c.doGet(url)
	if err != nil {
		return nil, err
	}

	var resp struct {
		OrganisationUnits []struct {
			ID string `json:"id"`
		} `json:"organisationUnits"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	uids := make([]string, len(resp.OrganisationUnits))
	for i, ou := range resp.OrganisationUnits {
		uids[i] = ou.ID
	}
	log.Printf("[DHIS2] Program has %d assigned org units", len(uids))
	return uids, nil
}

// deriveSectionPrefix extracts the section prefix from code or name.
func deriveSectionPrefix(code, name string) string {
	prefixes := []string{"ISS_SVC", "ISS_GEN", "ISS_EQ", "ISS_INFRA", "ISS_RH_SPE", "ISS_RH", "ISS_LAB", "ISS_COMMO"}
	// Check code first
	for _, p := range prefixes {
		if strings.HasPrefix(code, p+"_") || strings.HasPrefix(code, p) {
			return p
		}
	}
	// Check name prefix patterns
	for _, p := range prefixes {
		if strings.Contains(name, p) {
			return p
		}
	}
	// Special cases: ISS_NB_* are equipment
	if strings.HasPrefix(code, "ISS_NB_") {
		return "ISS_EQ"
	}
	// ISS_STATUT / ISS_PRIV are GEN
	if strings.HasPrefix(code, "ISS_STATUT") || strings.HasPrefix(code, "ISS_PRIV") {
		return "ISS_GEN"
	}
	// ISS_MEDECIN, ISS_INFORMATICIEN, ISS_AUTRE are RH
	if strings.HasPrefix(code, "ISS_MEDECIN") || strings.HasPrefix(code, "ISS_INFORMATICIEN") || strings.HasPrefix(code, "ISS_AUTRE") {
		return "ISS_RH"
	}
	return ""
}
