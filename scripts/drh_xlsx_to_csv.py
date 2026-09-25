#!/usr/bin/env python3
"""Convertit le fichier annuel de la DRH (.xlsx) en CSV normalisé pour l'import ISS.

    python3 scripts/drh_xlsx_to_csv.py "CNPS DRH 2026.xlsx" data/drh-2026.csv
    python3 scripts/drh_xlsx_to_csv.py "CNPS DRH 2026.xlsx" data/drh-2026.csv.gz   # compressé

Ne conserve que ce dont la carte sanitaire a besoin : aucune donnée identifiante
(ni matricule, ni nom, ni date de naissance exacte — seule l'année est gardée).
Le format produit est décrit dans docs/drh-format.md.

Une destination en .gz est ecrite compressee : le fichier annuel fait ~1,4 Mo,
au-dessus de la limite d'envoi par defaut des reverse proxies, et ~70 Ko gzippe.
"""
import csv, gzip, re, sys, unicodedata
from datetime import datetime

COLONNES = ['region','prefecture','sous_prefecture','structure_affectation','structure_rattachement',
            'profession','profession_oms','hierarchie','statut','sexe','annee_naissance','zone','niveau_structure']

# En-têtes acceptés dans le fichier DRH → colonne normalisée (insensible à la casse et aux accents)
SOURCE = {
    'region':'region', 'prefecture commune':'prefecture', 'sous prefecture':'sous_prefecture',
    'structure affectation':'structure_affectation', 'structure':'structure_rattachement',
    'profession':'profession', 'profession cnps oms':'profession_oms', 'hierarchie':'hierarchie',
    'statut employe':'statut', 'sexe':'sexe', 'date de naissance':'annee_naissance',
    'zone':'zone', 'niveau structure':'niveau_structure',
}

def norm(s):
    s = unicodedata.normalize('NFD', str(s or '').lower())
    s = ''.join(c for c in s if unicodedata.category(c) != 'Mn')
    return re.sub(r'[^a-z0-9]+', ' ', s).strip()

def clean_sexe(v):
    k = norm(v)
    return 'F' if k.startswith('f') else ('H' if k.startswith('h') or k.startswith('m') else '')

def clean_zone(v):
    k = norm(v)
    return 'rurale' if 'rural' in k else ('urbaine' if 'urbain' in k else '')

def clean_niveau(v):
    k = norm(v)
    for mot, out in (('tertiaire','tertiaire'), ('secondaire','secondaire'), ('primaire','primaire')):
        if mot in k:
            return out
    return ''

def annee(v):
    if isinstance(v, datetime):
        return v.year
    m = re.search(r'(19|20)\d{2}', str(v or ''))
    return int(m.group(0)) if m else ''

def main(src, dst):
    import openpyxl
    wb = openpyxl.load_workbook(src, read_only=True, data_only=True)
    ws = wb['BASE'] if 'BASE' in wb.sheetnames else wb[wb.sheetnames[0]]

    header, idx = None, {}
    rows_out, ignored = [], 0
    for raw in ws.iter_rows(values_only=True):
        if header is None:
            # la ligne d'en-tête est la première qui contient « Profession » et « Région »
            vals = [norm(c) for c in raw]
            if 'profession' in vals and 'region' in vals:
                header = vals
                for i, h in enumerate(header):
                    if h in SOURCE:
                        idx[SOURCE[h]] = i
                manquantes = [c for c in ('region','prefecture','profession','sexe') if c not in idx]
                if manquantes:
                    sys.exit(f"Colonnes obligatoires absentes du fichier : {', '.join(manquantes)}")
            continue
        if not any(v not in (None, '') for v in raw):
            continue
        get = lambda c: (str(raw[idx[c]]).strip() if c in idx and raw[idx[c]] not in (None, '') else '')
        rec = {c: get(c) for c in COLONNES}
        rec['sexe'] = clean_sexe(get('sexe'))
        rec['zone'] = clean_zone(get('zone'))
        rec['niveau_structure'] = clean_niveau(get('niveau_structure'))
        rec['annee_naissance'] = annee(raw[idx['annee_naissance']] if 'annee_naissance' in idx else '')
        if not rec['region'] and not rec['prefecture'] and not rec['profession']:
            ignored += 1
            continue
        rows_out.append(rec)

    opener = gzip.open if dst.endswith('.gz') else open
    with opener(dst, 'wt', newline='', encoding='utf-8-sig') as f:
        w = csv.DictWriter(f, fieldnames=COLONNES, delimiter=';')
        w.writeheader()
        w.writerows(rows_out)

    sans_annee = sum(1 for r in rows_out if not r['annee_naissance'])
    sans_struct = sum(1 for r in rows_out if not r['structure_affectation'] and not r['structure_rattachement'])
    print(f"{len(rows_out)} agents écrits dans {dst}")
    print(f"  colonnes trouvées : {len(idx)}/{len(COLONNES)}")
    print(f"  sans année de naissance : {sans_annee}   sans libellé de structure : {sans_struct}   lignes vides ignorées : {ignored}")
    print("  aucune donnée identifiante conservée (ni matricule, ni nom, ni date de naissance exacte)")

if __name__ == '__main__':
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
