# Format d'import du personnel (DRH)

Ce document décrit le **CSV normalisé** que la carte sanitaire sait importer pour le volet
« Personnel ». C'est le seul format accepté : le `.xlsx` annuel de la DRH n'est jamais lu
directement par l'application, il est converti au préalable (voir § Conversion).

## Principe

La DRH produit un fichier **nominatif** (matricule, nom, date de naissance exacte, poste).
La carte sanitaire n'a besoin d'**aucune** de ces données : elle ne publie que des effectifs
agrégés. Le CSV normalisé est donc une **projection dépersonnalisée** du fichier source :
une ligne par agent, mais aucune colonne permettant d'identifier cet agent.

| Retiré à la conversion | Pourquoi |
|---|---|
| Matricule | Identifiant direct |
| Nom, prénom | Identifiant direct |
| Date de naissance exacte | Quasi-identifiant ; seule l'**année** est conservée (tranches quinquennales) |
| Poste occupé, service, téléphone | Non utilisés par les agrégats, ré-identifiants combinés au reste |

Aucune ligne individuelle n'est stockée en base : l'import calcule les agrégats puis ne
conserve que ceux-ci (voir `.claude/plan-personnel-drh.md`, décision 1). Le CSV lui-même reste
un fichier de travail, à **ne pas committer** (`data/` est dans `.gitignore`).

## Structure du fichier

- Encodage **UTF-8** (BOM toléré), séparateur **`;`**, fin de ligne `\n` ou `\r\n`.
- Première ligne = en-tête, avec les noms de colonnes exacts ci-dessous, dans cet ordre.
- Une ligne par agent. Les cellules vides sont admises (voir « Obligatoire »).

| # | Colonne | Obligatoire | Valeurs attendues |
|---|---|---|---|
| 1 | `region` | oui | Libellé de la région administrative (11 valeurs : BOKE, CONAKRY, FARANAH, …) |
| 2 | `prefecture` | oui | Préfecture ou commune de Conakry (48 valeurs) — sert au rattachement au district ISS |
| 3 | `sous_prefecture` | non | Sous-préfecture ou commune urbaine |
| 4 | `structure_affectation` | non | Libellé de la structure d'affectation, tel qu'écrit par la DRH (ex. `CS KOULE`, `HRKkan`). C'est la clé du rattachement aux structures ISS |
| 5 | `structure_rattachement` | non | Structure de rattachement administratif (ex. `DPS Siguiri`). Sert de repli quand la colonne 4 est vide |
| 6 | `profession` | oui | Intitulé DRH du métier (112 valeurs : `Médécin Généraliste`, `Infirmier d'Etat`, `ATS`, …) |
| 7 | `profession_oms` | non | Regroupement OMS de la profession (`Médecin Généraliste (medecins de famille compris)`, `Autres spécialistes`, …) |
| 8 | `hierarchie` | non | Catégorie de la fonction publique : `A1`, `A2`, `A3`, `B1`, `B2`, `C`, `D` |
| 9 | `statut` | non | `Fonctionnaire`, `Contractuel Permanent`, `Contractuel Temporaire`, `Contractuel d'Etat` |
| 10 | `sexe` | oui | **`F`** ou **`H`** (normalisé à la conversion) |
| 11 | `annee_naissance` | non | Année sur 4 chiffres (ex. `1981`). Vide si inconnue → l'agent est exclu de la pyramide des âges et des départs à la retraite, mais compté dans les effectifs |
| 12 | `zone` | non | `urbaine` ou `rurale` |
| 13 | `niveau_structure` | non | `primaire`, `secondaire` ou `tertiaire` |

Toute colonne supplémentaire est **rejetée** à l'import : c'est la garantie qu'aucune donnée
identifiante n'entre par inadvertance. Une colonne obligatoire vide fait rejeter la ligne, qui
est comptée dans le rapport d'import.

### Exemple

```csv
region;prefecture;sous_prefecture;structure_affectation;structure_rattachement;profession;profession_oms;hierarchie;statut;sexe;annee_naissance;zone;niveau_structure
BOKE;Boké;Commune Urbaine;IRS Boké;IRS Boké;Médecin Spécialiste en Santé Publique;Autres spécialistes;A2;Fonctionnaire;H;1981;urbaine;secondaire
KANKAN;Kankan;Kankan Centre;HRKkan;HR Kankan;Infirmier d'Etat;Personnel infirmier;B2;Fonctionnaire;F;1990;urbaine;secondaire
```

## Conversion depuis le `.xlsx` de la DRH

```bash
python3 scripts/drh_xlsx_to_csv.py "CNPS DRH 2026.xlsx" data/drh-2026.csv
python3 scripts/drh_xlsx_to_csv.py "CNPS DRH 2026.xlsx" data/drh-2026.csv.gz   # version compressée
```

### Doublons

Le fichier 2026 contient **125 lignes saisies deux fois** — même matricule, même contenu, date de naissance
comprise. Le convertisseur les supprime : un agent compté deux fois gonfle les effectifs et les densités
(sur 2026 : −50 infirmiers, −32 sages-femmes, −29 ATS, 10 162 agents ramenés à **10 037**).

Il reste **259 matricules portés par des lignes qui diffèrent** (structure, date de naissance, profession…).
Ceux-là sont **conservés** : la même immatriculation sur deux lignes différentes peut être une faute de saisie
comme deux personnes distinctes, et trancher n'appartient pas à l'application. `--doublons <fichier.csv>` écrit
la liste à renvoyer à la DRH.

Les matricules de remplissage (`ND`, `0`, `N/A`…) ne sont jamais traités comme des identifiants : dans le
millésime 2026, `ND` porte quinze agents différents.

Une destination en `.gz` est écrite compressée, et l'import accepte les deux. Le millésime 2026 fait
**1,4 Mo en clair, 70 Ko gzippé** : au-delà de 1 Mo, le serveur web placé devant l'application refuse
souvent l'envoi (`413 Request Entity Too Large`, limite `client_max_body_size` d'nginx) sans que la requête
atteigne l'application. Envoyer le `.csv.gz` évite d'avoir à toucher à cette configuration.

Le script lit l'onglet `BASE`, repère la ligne d'en-tête (celle contenant « Profession » et
« Région »), mappe les colonnes sources vers les colonnes normalisées, normalise `sexe`,
`zone` et `niveau_structure`, ne garde que l'**année** de la date de naissance, et **n'écrit
jamais** matricule ni nom. Il affiche à la fin le nombre d'agents convertis et le nombre de
valeurs manquantes.

Si la DRH renomme une colonne d'une année sur l'autre, ajouter l'en-tête dans le dictionnaire
`SOURCE` en tête de script (la comparaison est insensible à la casse et aux accents). Une
colonne obligatoire introuvable fait échouer la conversion avec un message explicite.

## Aligner les noms à la source

La première cause de non-rattachement n'est pas l'absence de structure, c'est l'**orthographe** : la DRH écrit
« CSR Damakania », « Hopital Regional Kindia », « HRKkan » là où DHIS2 a « CSR Damankanya », « HR Kindia »,
« HR Kankan ». Sur le millésime 2026, **466 libellés sur 986** diffèrent du nom DHIS2.

Rattraper ces écarts dans l'application a un coût qui revient chaque année. Le corriger **à la source** est
définitif. D'où la feuille de correction :

```bash
cd backend
DRHCHECK_CORRECTIONS="DRH 2026 - noms a corriger.csv" \
  go run ./cmd/drhcheck /chemin/copie-de-iss.db drh-2026.csv correspondances.csv 2026
```

Elle liste chaque libellé du fichier avec le nom DHIS2 attendu :

| colonne | contenu |
|---|---|
| `libelle_drh` | ce que la DRH a écrit |
| `prefecture` | la préfecture de l'agent, qui lève les homonymies |
| `n_agents` | combien d'agents sont concernés |
| `nom_dhis2_attendu` | le nom exact à reprendre, ou `(bureau de district)`, `(administration centrale)`, ou **`À PRÉCISER`** |
| `uid_dhis2` | l'identifiant de l'unité d'organisation |
| `reconnu_par` | comment le rattachement a été obtenu (`table`, `exact`, `approx`, `deduit`, `prefixe`…) |

Les lignes `À PRÉCISER` sont en tête : ce sont celles que personne ne peut résoudre sans connaître le terrain —
libellés génériques (« CS », « Centre de Santé »), sigles non documentés, structures absentes du recensement.

Une fois la source alignée, la table de correspondance ne garde que les cas réellement ambigus, et les règles
de code se limitent à ce qui est structurel : bureaux de district, administration centrale, services hébergés
par l'hôpital du district.

## Rattachement aux structures ISS

`structure_affectation` est un texte libre saisi par la DRH : il ne correspond pas toujours au
nom ISS. Le rattachement se fait dans cet ordre, à l'import :

1. **Table de correspondance** `data/DRH - correspondances structures.csv` — les cas validés à
   la main. Elle est éditable depuis l'écran d'administration (*Remplacer les correspondances*)
   et fait autorité.

   ```
   libelle_drh;structure_iss;uid_dhis2;district;type;statut
   CSR DAMAKANIA;CSR Damankanya;laCYbXGU2pv;DPS Kindia;CS;OK
   # cette ligne est ignorée : un # en tête désactive une règle sans la perdre
   ```

   | Colonne | Rôle |
   |---|---|
   | `libelle_drh` | le libellé tel que la DRH l'écrit. Casse et accents sont normalisés, une seule ligne suffit pour `CS LEYSARE` et `CS Leysaré` |
   | `structure_iss` | le nom de la structure, pour la lecture humaine — l'appariement se fait sur l'UID |
   | `uid_dhis2` | **ce qui compte** : l'identifiant de l'unité d'organisation, recensée ou non |
   | `district` | **où la règle s'applique**. Renseigné, elle est limitée à ce district (« HOPITAL » désigne HP Fria à Fria et rien ailleurs) ; vide, elle vaut partout |
   | `type` | informatif |
   | `statut` | `OK`, `bureau de district`, `non rattache`, `a trancher` |

   Deux lignes pour le meme libelle : **la derniere gagne**. La table s'edite en ajoutant a la fin, donc une
   correction ecrite apres coup l'emporte sur la regle qu'elle corrige.
2. **Nom normalisé identique** à une structure ISS du même district.
3. **Type + nom propre** : le type est déduit du libellé (`HR`, `HP`, `CMC`, `CSA`, `CS`, `PS`)
   et sert de discriminant entre structures homonymes du district.
4. **Déduction** : un seul établissement de ce type dans le district → rattachement.
5. **Bureau de district** (`DPS`, `DCS`, `IRS`, `DSP`) : compté dans la densité du district,
   sans structure.
6. **Administration centrale** : compté au niveau national uniquement.

Ce qui ne tombe dans aucun cas reste **non rattaché** et apparaît tel quel dans le rapport
d'import, pour arbitrage. Le fichier 2026 est couvert à 99,8 % (16 agents non rattachés).
