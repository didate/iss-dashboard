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
 *  Le fond reste OSM en connaissance de cause. Esri a été essayé, et écarté :
 *  il ne cartographie pas le bâti des villes de l'intérieur — à Kankan, au zoom
 *  17, sa tuile ne porte que le nom de la ville et une voie ferrée, là où OSM
 *  montre les rues et les maisons. Or chercher une structure dans un quartier
 *  est l'usage principal de la carte publique.
 *
 *  Reste que la politique d'OSM déconseille ses serveurs bénévoles pour un
 *  service en production : la réponse durable est un fournisseur des mêmes
 *  données avec engagement de service (MapTiler, Stadia), le jour où le
 *  Ministère disposera d'une clé. `VITE_TILE_URL` et `VITE_TILE_ATTRIBUTION`
 *  suffisent alors à basculer, sans toucher au code. Sans clé :
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
