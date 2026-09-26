package drh

// QualiteDistrict signale un district dont le fichier DRH ne dit pas où
// travaillent les agents : leur libellé d'affectation est le bureau de
// district (« DPS », « DPS KISSIDOUGOU ») et non la structure de soins.
//
// Le critère est volontairement grossier et invérifiable autrement : plus
// d'agents au bureau que dans toutes les structures du district réunies. Aucune
// organisation réelle ne ressemble à ça — un DPS n'emploie pas plus de monde que
// l'ensemble des hôpitaux et centres de santé qu'il supervise. Quand le compte
// tombe de ce côté, ce n'est donc pas un fait de terrain mais une saisie trop
// vague, et on le dit.
//
// Ces districts ne sont pas écartés des calculs : leurs agents restent comptés
// dans les effectifs, et leur comparaison avec ISS reste affichée. Elle est
// simplement signalée pour ce qu'elle vaut — un artefact du fichier, à renvoyer
// à la DRH, pas un résultat sur le district.
type QualiteDistrict struct {
	District   string  `json:"district"`
	Region     string  `json:"region"`
	NAgents    int     `json:"n_agents"`
	NStructure int     `json:"n_structure"`
	NBureau    int     `json:"n_bureau"`
	PctBureau  float64 `json:"pct_bureau"`
}
