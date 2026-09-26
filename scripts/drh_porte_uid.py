#!/usr/bin/env python3
"""Porte les colonnes DHIS2 du classeur anonymisé enrichi vers le fichier
d'origine de la DRH — celui qui porte les matricules, et qui seul permet le
dédoublonnage.

    python3 scripts/drh_porte_uid.py <original.xlsx> <enrichi.xlsx> <sortie.xlsx>

L'original est nominatif : il reste hors du dépôt, et la sortie aussi. Seul le
CSV produit ensuite par drh_xlsx_to_csv.py, qui ne garde aucune donnée
identifiante, entre dans data/.

Les deux classeurs descendent du même fichier DRH, à la colonne Matricule près :
le report se fait ligne à ligne. L'alignement est vérifié sur toutes les
colonnes communes avant la moindre écriture — au premier écart, rien n'est écrit.
"""
import sys
from openpyxl import load_workbook
from openpyxl.styles import Font, PatternFill
from openpyxl.utils import get_column_letter

HDR = 2                                            # ligne d'en-tête du classeur DRH
COLS = ["nom_dhis2", "uid_dhis2", "rattachement"]


def main(orig, enrichi, dst):
    wbe = load_workbook(enrichi)
    wse = wbe["BASE"]
    he = {c.value: c.column for c in wse[HDR] if c.value}
    manquantes = [c for c in COLS if c not in he]
    if manquantes:
        sys.exit(f"colonnes absentes du classeur enrichi : {', '.join(manquantes)}")

    wbo = load_workbook(orig)
    wso = wbo["BASE"]
    ho = {c.value: c.column for c in wso[HDR] if c.value}

    if wso.max_row != wse.max_row:
        sys.exit(f"nombre de lignes différent : {wso.max_row} vs {wse.max_row}")
    communes = [h for h in ho if h in he and h not in COLS]
    ecarts = 0
    for r in range(HDR + 1, wso.max_row + 1):
        for h in communes:
            a, b = wso.cell(r, ho[h]).value, wse.cell(r, he[h]).value
            # une cellule trop étroite est mise en cache sous la forme « #### » :
            # c'est un artefact d'affichage, pas une divergence de contenu.
            if a != b and not (isinstance(a, str) and set(a) == {"#"}):
                ecarts += 1
                break
    if ecarts:
        sys.exit(f"{ecarts} lignes divergentes : alignement non garanti, rien n'est écrit")

    base = max(ho.values())      # les nouvelles colonnes se posent après la dernière remplie
    gras = Font(bold=True)
    for k, nom in enumerate(COLS):
        col = base + 1 + k
        cell = wso.cell(HDR, col, nom)
        cell.font = gras
        cell.fill = PatternFill("solid", fgColor="DDEBF7")
        wso.column_dimensions[get_column_letter(col)].width = 28 if k < 2 else 18

    sans_uid = 0
    for r in range(HDR + 1, wso.max_row + 1):
        for k, nom in enumerate(COLS):
            ce = wse.cell(r, he[nom])
            co = wso.cell(r, base + 1 + k)
            co.value = ce.value
            if ce.fill and ce.fill.patternType:      # garde le code couleur de provenance
                co.fill = PatternFill("solid", fgColor=ce.fill.fgColor.rgb)
        if not wse.cell(r, he["uid_dhis2"]).value:
            sans_uid += 1

    wso.auto_filter.ref = f"A{HDR}:{get_column_letter(base + len(COLS))}{wso.max_row}"
    wso.sheet_view.topLeftCell = "A1"                # sinon le classeur s'ouvre où il était
    wso.freeze_panes = f"A{HDR + 1}"
    wbo.save(dst)

    cols = "/".join(get_column_letter(base + 1 + k) for k in range(len(COLS)))
    print(f"écrit : {dst}")
    print(f"  colonnes {cols} · {wso.max_row - HDR} lignes · sans identifiant DHIS2 : {sans_uid}")


if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    main(*sys.argv[1:])
