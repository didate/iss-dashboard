// Libellés d'affichage de la typologie (miroir de backend/internal/typologie).
// Le backend renvoie déjà type_label partout où c'est possible ; ceci sert aux
// endroits où seul le code circule (détail d'event, agrégats par clé).
const LABELS: Record<string, string> = {
  PS: 'Poste de santé',
  CS: 'Centre de santé',
  CSA: 'Centre de santé amélioré',
  CMC: 'Centre médico-communal',
  HP: 'Hôpital préfectoral',
  HR: 'Hôpital régional',
  HN: 'Hôpital national',
  CABINET: 'Cabinet médical',
  CLINIQUE: 'Clinique / centre médical',
  AUTRE_PRIVE: 'Autre structure privée',
  ASC: 'Site communautaire',
  INDETERMINE: 'Type indéterminé',
};

export function typologieLabel(code: string): string {
  return LABELS[code] ?? code;
}

export function typeSourceLabel(source: string): string {
  switch (source) {
    case 'group':
      return 'groupe DHIS2';
    case 'group_multiple':
      return 'plusieurs groupes DHIS2';
    case 'name':
      return 'déduit du nom';
    default:
      return 'indéterminé';
  }
}
