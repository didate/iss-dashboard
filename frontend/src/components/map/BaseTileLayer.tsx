import { TileLayer } from 'react-leaflet';

/** Fond de carte, en un seul endroit pour les trois cartes de l'application.
 *
 *  Les serveurs de tuiles d'OpenStreetMap ont bloqué l'application en
 *  production — chaque tuile remplacée par une image « Access blocked — App is
 *  not following the tile usage policy ». La cause est l'URL historique à
 *  sous-domaines `{s}.tile.openstreetmap.org`, dépréciée par OSM et désormais
 *  traitée comme une violation de sa politique d'usage : le domaine unique
 *  `tile.openstreetmap.org` est le seul supporté.
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
  return <TileLayer attribution={attribution} url={url} maxZoom={19} />;
}
