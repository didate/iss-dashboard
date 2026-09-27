import { TileLayer } from 'react-leaflet';

/** Fond de carte, en un seul endroit pour les trois cartes de l'application.
 *
 *  Les serveurs de tuiles d'OpenStreetMap renvoient une image « Access blocked
 *  — App is not following the tile usage policy » à toute requête qui ne
 *  s'identifie pas. Vérifié : la même tuile demandée avec un en-tête `Referer`
 *  revient en carte, demandée sans lui revient en 403 illustré, octet pour
 *  octet identique quelles que soient les coordonnées.
 *
 *  Or la production répond `Referrer-Policy: no-referrer` : le navigateur
 *  n'envoie donc aucun référent, et OSM bloque. Le symptôme n'apparaît pas
 *  partout — un navigateur qui a gardé les tuiles en cache continue de les
 *  afficher — ce qui le fait passer pour un problème de navigateur alors que
 *  c'est un en-tête du serveur.
 *
 *  `referrerPolicy` posé sur les tuiles l'emporte sur la politique du
 *  document : les images partent avec l'origine du site pour seul référent,
 *  assez pour qu'OSM identifie l'application, sans divulguer le chemin ni les
 *  paramètres de la page — un identifiant de structure, par exemple.
 *
 *  `VITE_TILE_URL` et `VITE_TILE_ATTRIBUTION` permettent de basculer vers un
 *  autre fournisseur sans toucher au code, si les tuiles bénévoles d'OSM
 *  s'avéraient trop fragiles pour un service public. Deux fonds sans clé :
 *
 *    Esri   https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}
 *    OSM-FR https://{s}.tile.openstreetmap.fr/osmfr/{z}/{x}/{y}.png
 *
 *  (CARTO exige désormais une clé : ses tuiles « gratuites » renvoient une
 *  image filigranée « API KEY REQUIRED ».)
 */
const DEFAULT_URL = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
const DEFAULT_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>';

const url = import.meta.env.VITE_TILE_URL || DEFAULT_URL;
const attribution = import.meta.env.VITE_TILE_ATTRIBUTION || DEFAULT_ATTRIBUTION;

export default function BaseTileLayer() {
  return <TileLayer attribution={attribution} url={url} maxZoom={19} referrerPolicy="strict-origin-when-cross-origin" />;
}
