import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { MapContainer, TileLayer, GeoJSON } from 'react-leaflet';
import type { Layer, PathOptions } from 'leaflet';
import type { Feature, Geometry } from 'geojson';
import { api } from '../api/client';
import type { MapDistrictCollection, MapDistrictProperties } from '../types';

type LayerKey = 'rapportage' | 'qualite' | 'services' | 'equipements' | 'wash' | 'rh';

const LAYERS: { key: LayerKey; label: string }[] = [
  { key: 'rapportage', label: 'Taux de rapportage' },
  { key: 'qualite', label: 'Score qualite' },
  { key: 'services', label: 'Couverture services' },
  { key: 'equipements', label: 'Equipements' },
  { key: 'wash', label: 'WASH (Forage/Reseau)' },
  { key: 'rh', label: 'Densite RH' },
];

function getColorPct(value: number | null): string {
  if (value === null || value === undefined) return '#d1d5db';
  if (value < 30) return '#ef4444';
  if (value < 50) return '#f97316';
  if (value < 80) return '#eab308';
  return '#22c55e';
}

function getColorScore(value: number | null): string {
  if (value === null || value === undefined) return '#d1d5db';
  if (value < 50) return '#ef4444';
  if (value < 65) return '#f97316';
  if (value < 80) return '#eab308';
  return '#22c55e';
}

function getColorRH(value: number | null): string {
  if (value === null || value === undefined) return '#d1d5db';
  if (value < 0.2) return '#ef4444';
  if (value < 0.5) return '#f97316';
  if (value < 1) return '#eab308';
  return '#22c55e';
}

function computeQuantileBreaks(values: number[], n: number): number[] {
  const sorted = [...values].filter(v => v > 0).sort((a, b) => a - b);
  if (sorted.length === 0) return [];
  const breaks: number[] = [];
  for (let i = 1; i < n; i++) {
    const idx = Math.floor((i / n) * sorted.length);
    breaks.push(sorted[Math.min(idx, sorted.length - 1)]);
  }
  return breaks;
}

function getColorQuantile(value: number | null | undefined, breaks: number[]): string {
  if (value === null || value === undefined || value === 0) return '#d1d5db';
  const colors = ['#ef4444', '#f97316', '#eab308', '#84cc16', '#22c55e'];
  for (let i = 0; i < breaks.length; i++) {
    if (value <= breaks[i]) return colors[i];
  }
  return colors[colors.length - 1];
}

interface LegendItem {
  color: string;
  label: string;
}

function getLegendItems(layer: LayerKey, breaks?: number[]): LegendItem[] {
  if (layer === 'rapportage' || layer === 'services' || layer === 'wash') {
    return [
      { color: '#ef4444', label: '< 30%' },
      { color: '#f97316', label: '30-50%' },
      { color: '#eab308', label: '50-80%' },
      { color: '#22c55e', label: '> 80%' },
      { color: '#d1d5db', label: 'Pas de donnees' },
    ];
  }
  if (layer === 'qualite') {
    return [
      { color: '#ef4444', label: '< 50' },
      { color: '#f97316', label: '50-65' },
      { color: '#eab308', label: '65-80' },
      { color: '#22c55e', label: '> 80' },
      { color: '#d1d5db', label: 'Pas de donnees' },
    ];
  }
  if (layer === 'rh') {
    return [
      { color: '#ef4444', label: '< 0.2 med/struct' },
      { color: '#f97316', label: '0.2 - 0.5' },
      { color: '#eab308', label: '0.5 - 1' },
      { color: '#22c55e', label: '> 1' },
      { color: '#d1d5db', label: 'Pas de donnees' },
    ];
  }
  if (layer === 'equipements' && breaks && breaks.length > 0) {
    const labels = [`<= ${breaks[0]}`];
    for (let i = 1; i < breaks.length; i++) {
      labels.push(`${breaks[i - 1] + 1} - ${breaks[i]}`);
    }
    labels.push(`> ${breaks[breaks.length - 1]}`);
    const colors = ['#ef4444', '#f97316', '#eab308', '#84cc16', '#22c55e'];
    const items = labels.map((l, i) => ({ color: colors[i] || '#22c55e', label: l }));
    items.push({ color: '#d1d5db', label: 'Pas de donnees' });
    return items;
  }
  return [{ color: '#d1d5db', label: 'Pas de donnees' }];
}

export default function MapView() {
  const [data, setData] = useState<MapDistrictCollection | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [activeLayer, setActiveLayer] = useState<LayerKey>('rapportage');
  const [selectedService, setSelectedService] = useState('');
  const [selectedEquipCategory, setSelectedEquipCategory] = useState('');
  const geoJsonRef = useRef<L.GeoJSON | null>(null);

  useEffect(() => {
    api.getMapData()
      .then(setData)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  // Derive available services and equipment categories from data
  const serviceOptions = useMemo(() => {
    if (!data) return [];
    const allServices = new Map<string, string>();
    for (const f of data.features) {
      for (const [code, svc] of Object.entries(f.properties.services || {})) {
        if (!allServices.has(code)) allServices.set(code, svc.service_label);
      }
    }
    return Array.from(allServices.entries()).sort((a, b) => a[1].localeCompare(b[1]));
  }, [data]);

  const equipCategories = useMemo(() => {
    if (!data) return [];
    const cats = new Set<string>();
    for (const f of data.features) {
      for (const eq of Object.values(f.properties.equipements || {})) {
        if (eq.category) cats.add(eq.category);
      }
    }
    return Array.from(cats).sort();
  }, [data]);

  // Set defaults when data loads
  useEffect(() => {
    if (serviceOptions.length > 0 && !selectedService) {
      setSelectedService(serviceOptions[0][0]);
    }
  }, [serviceOptions, selectedService]);

  useEffect(() => {
    if (equipCategories.length > 0 && !selectedEquipCategory) {
      setSelectedEquipCategory(equipCategories[0]);
    }
  }, [equipCategories, selectedEquipCategory]);

  // Quantile breaks for equipment layer
  const equipBreaks = useMemo(() => {
    if (!data || activeLayer !== 'equipements' || !selectedEquipCategory) return [];
    const values: number[] = [];
    for (const f of data.features) {
      let total = 0;
      for (const eq of Object.values(f.properties.equipements || {})) {
        if (eq.category === selectedEquipCategory) total += eq.sum_total;
      }
      values.push(total);
    }
    return computeQuantileBreaks(values, 5);
  }, [data, activeLayer, selectedEquipCategory]);

  const getFeatureValue = useCallback((props: MapDistrictProperties): { value: number | null; display: string } => {
    switch (activeLayer) {
      case 'rapportage':
        return {
          value: props.rapportage_pct,
          display: props.rapportage_pct !== null ? `${props.rapportage_pct.toFixed(1)}% (${props.rapportage_reported}/${props.rapportage_expected})` : 'N/A',
        };
      case 'qualite':
        return {
          value: props.qualite_avg_score,
          display: props.qualite_avg_score !== null ? `${props.qualite_avg_score.toFixed(1)} (${props.qualite_n_structures} struct.)` : 'N/A',
        };
      case 'services': {
        const svc = props.services?.[selectedService];
        if (!svc) return { value: null, display: 'N/A' };
        return { value: svc.pct_fonctionnel, display: `${svc.pct_fonctionnel.toFixed(1)}% (${svc.n_oui}/${svc.n_total})` };
      }
      case 'equipements': {
        let total = 0, fonct = 0;
        for (const eq of Object.values(props.equipements || {})) {
          if (eq.category === selectedEquipCategory) {
            total += eq.sum_total;
            fonct += eq.sum_fonct;
          }
        }
        if (total === 0 && fonct === 0) return { value: null, display: 'N/A' };
        return { value: total, display: `Total: ${total}, Fonct: ${fonct}` };
      }
      case 'wash':
        return {
          value: props.wash_forage_ou_reseau_pct,
          display: props.wash_forage_ou_reseau_pct !== null
            ? `${props.wash_forage_ou_reseau_pct.toFixed(1)}% (${props.wash_forage_ou_reseau_n}/${props.wash_total})`
            : 'N/A',
        };
      case 'rh':
        return {
          value: props.rh_medecins_par_structure,
          display: props.rh_medecins_par_structure !== null
            ? `${props.rh_medecins_par_structure.toFixed(2)} med/struct (${props.rh_medecins_total} med, ${props.rh_n_structures} struct.)`
            : 'N/A',
        };
      default:
        return { value: null, display: 'N/A' };
    }
  }, [activeLayer, selectedService, selectedEquipCategory]);

  const getColor = useCallback((value: number | null): string => {
    switch (activeLayer) {
      case 'rapportage':
      case 'services':
      case 'wash':
        return getColorPct(value);
      case 'qualite':
        return getColorScore(value);
      case 'rh':
        return getColorRH(value);
      case 'equipements':
        return getColorQuantile(value, equipBreaks);
      default:
        return '#d1d5db';
    }
  }, [activeLayer, equipBreaks]);

  const style = useCallback((feature: Feature<Geometry, MapDistrictProperties> | undefined): PathOptions => {
    if (!feature) return { fillColor: '#d1d5db', weight: 1, color: '#6b7280', fillOpacity: 0.7 };
    const { value } = getFeatureValue(feature.properties);
    return {
      fillColor: getColor(value),
      weight: 1.5,
      color: '#374151',
      fillOpacity: 0.7,
    };
  }, [getFeatureValue, getColor]);

  const onEachFeature = useCallback((feature: Feature<Geometry, MapDistrictProperties>, layer: Layer) => {
    const props = feature.properties;
    const { display } = getFeatureValue(props);

    layer.bindTooltip(`<strong>${props.district_name}</strong><br/>${display}`, {
      sticky: true,
      className: 'map-tooltip',
    });

    layer.on({
      mouseover: (e) => {
        const target = e.target;
        target.setStyle({ weight: 3, color: '#1d4ed8', fillOpacity: 0.85 });
        target.bringToFront();
      },
      mouseout: (e) => {
        if (geoJsonRef.current) {
          geoJsonRef.current.resetStyle(e.target);
        }
      },
    });
  }, [getFeatureValue]);

  // Force GeoJSON re-render when layer/selection changes
  const geoJsonKey = `${activeLayer}-${selectedService}-${selectedEquipCategory}`;

  if (loading) return <div className="p-6 text-gray-500">Chargement de la carte...</div>;
  if (error) return <div className="p-6 text-red-500">Erreur : {error}</div>;
  if (!data || data.features.length === 0) {
    return <div className="p-6 text-gray-500">Aucune donnee geographique disponible. Lancez une synchronisation pour recuperer les contours des districts.</div>;
  }

  const legendItems = getLegendItems(activeLayer, equipBreaks);

  return (
    <div className="h-full flex flex-col">
      {/* Layer tabs */}
      <div className="flex flex-wrap gap-1 mb-3">
        {LAYERS.map(l => (
          <button
            key={l.key}
            onClick={() => setActiveLayer(l.key)}
            className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
              activeLayer === l.key
                ? 'bg-blue-600 text-white'
                : 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50'
            }`}
          >
            {l.label}
          </button>
        ))}
      </div>

      {/* Sub-selectors */}
      {activeLayer === 'services' && serviceOptions.length > 0 && (
        <div className="mb-3">
          <select
            value={selectedService}
            onChange={e => setSelectedService(e.target.value)}
            className="border border-gray-300 rounded px-3 py-1.5 text-sm bg-white"
          >
            {serviceOptions.map(([code, label]) => (
              <option key={code} value={code}>{label}</option>
            ))}
          </select>
        </div>
      )}
      {activeLayer === 'equipements' && equipCategories.length > 0 && (
        <div className="mb-3">
          <select
            value={selectedEquipCategory}
            onChange={e => setSelectedEquipCategory(e.target.value)}
            className="border border-gray-300 rounded px-3 py-1.5 text-sm bg-white"
          >
            {equipCategories.map(cat => (
              <option key={cat} value={cat}>{cat}</option>
            ))}
          </select>
        </div>
      )}

      {/* Map + Legend */}
      <div className="flex-1 relative rounded-lg overflow-hidden border border-gray-200" style={{ minHeight: '500px' }}>
        <MapContainer
          center={[10.5, -11.8]}
          zoom={7}
          style={{ height: '100%', width: '100%' }}
          scrollWheelZoom={true}
        >
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          <GeoJSON
            key={geoJsonKey}
            ref={(ref) => { geoJsonRef.current = ref; }}
            data={data as unknown as GeoJSON.FeatureCollection}
            style={style as (feature?: Feature) => PathOptions}
            onEachFeature={onEachFeature as (feature: Feature, layer: Layer) => void}
          />
        </MapContainer>

        {/* Legend */}
        <div className="absolute bottom-4 right-4 bg-white rounded-lg shadow-lg p-3 z-[1000]">
          <h4 className="text-xs font-semibold mb-2 text-gray-700">
            {LAYERS.find(l => l.key === activeLayer)?.label}
          </h4>
          {legendItems.map((item, i) => (
            <div key={i} className="flex items-center gap-2 text-xs text-gray-600 mb-1">
              <span
                className="inline-block w-4 h-3 rounded-sm border border-gray-300"
                style={{ backgroundColor: item.color }}
              />
              {item.label}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
