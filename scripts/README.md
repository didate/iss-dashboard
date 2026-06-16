# Scripts utilitaires ISS

## Prérequis

```bash
pip3 install openpyxl
```

## Export Excel

Exporte toutes les données ISS (structures, valeurs, scores qualité) dans un fichier Excel.

### Lancer une synchro avant l'export

```bash
# Via curl (remplacez le token par le vôtre)
curl -X POST http://localhost:8080/iss/api/admin/sync -H "X-Admin-Token: votre_token"

# Attendez ~30 secondes puis vérifiez le statut
curl -s http://localhost:8080/iss/api/admin/sync/status -H "X-Admin-Token: votre_token"
```

En production :
```bash
curl -X POST https://apps.sante.gov.gn/iss/api/admin/sync -H "X-Admin-Token: votre_token"
```

### Générer l'Excel

```bash
# Utilisation par défaut (DB locale : backend/data/iss.db)
python3 scripts/export_excel.py

# Spécifier la base de données
python3 scripts/export_excel.py --db /chemin/vers/iss.db

# Spécifier le fichier de sortie
python3 scripts/export_excel.py --output mon_rapport.xlsx

# Les deux
python3 scripts/export_excel.py --db /data/iss.db -o extraction.xlsx
```

Le fichier est généré à la racine du projet avec un horodatage : `extraction_iss_20260616_1154.xlsx`.

### Contenu du fichier

| Colonnes fixes | Description |
|---|---|
| Région | Région sanitaire |
| District | District sanitaire |
| Structure | Nom de la structure |
| Org Unit UID | Identifiant DHIS2 de l'org unit |
| Event UID | Identifiant DHIS2 de l'événement |
| Date | Date de l'événement |
| Statut | Statut DHIS2 (COMPLETED, etc.) |
| Score Qualité | Score 0-100 calculé par le moteur de règles |
| Erreurs | Nombre d'erreurs qualité |
| Avertissements | Nombre d'avertissements qualité |
| Infos | Nombre d'infos qualité |

Suivi de **225 colonnes** correspondant à chaque data element du programme ISS, avec le nom lisible en en-tête.
