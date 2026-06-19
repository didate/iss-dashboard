import { useEffect, useState, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell, PieChart, Pie, Legend } from 'recharts';
import { MapContainer, GeoJSON } from 'react-leaflet';
import type { PathOptions } from 'leaflet';
import type { Feature, Geometry } from 'geojson';
import { ArrowLeft, Download } from 'lucide-react';
import html2canvas from 'html2canvas';
import { jsPDF } from 'jspdf';
import { api } from '../api/client';
import type {
  UsageService, UsageEquipement, UsageRH, UsageCommodite,
  RHSummaryResult, ReportingRate, PlateauItem, MapDistrictCollection,
  QualitySummaryRow, IssueListResult, Filters,
} from '../types';

export default function DistrictReport() {
  const { district: districtUID } = useParams<{ district: string }>();
  const navigate = useNavigate();
  const reportRef = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [districtName, setDistrictName] = useState('');

  // Data
  const [qualitySummary, setQualitySummary] = useState<QualitySummaryRow | null>(null);
  const [reporting, setReporting] = useState<ReportingRate | null>(null);
  const [services, setServices] = useState<UsageService[]>([]);
  const [equipements, setEquipements] = useState<UsageEquipement[]>([]);
  const [rh, setRH] = useState<UsageRH[]>([]);
  const [rhSummary, setRHSummary] = useState<RHSummaryResult | null>(null);
  const [commodites, setCommodites] = useState<UsageCommodite[]>([]);
  const [plateau, setPlateau] = useState<PlateauItem[]>([]);
  const [issues, setIssues] = useState<IssueListResult | null>(null);
  const [mapData, setMapData] = useState<MapDistrictCollection | null>(null);

  useEffect(() => {
    if (!districtUID) return;
    setLoading(true);

    // First resolve UID → name
    api.getFilters().then((filters: Filters) => {
      const name = filters.district_uids?.[districtUID] || decodeURIComponent(districtUID);
      setDistrictName(name);

      // Then fetch all data using the name
      return Promise.all([
        api.getQualitySummary('district').then(rows => {
          setQualitySummary((rows ?? []).find(r => r.key === name) || null);
        }),
        api.getReportingRate('district').then(rows => {
          setReporting((rows ?? []).find(r => r.key === name) || null);
        }),
        api.getUsageServices(name).then(d => setServices(d ?? [])),
        api.getUsageEquipements('all', name).then(d => setEquipements(d ?? [])),
        api.getUsageRH(name).then(d => setRH(d ?? [])),
        api.getRHSummary(name).then(setRHSummary),
        api.getUsageCommodites(name).then(d => setCommodites(d ?? [])),
        api.getPlateauTechnique(name).then(d => setPlateau(d ?? [])),
        api.getQualityIssues({ district: name, page: 1, pageSize: 20 }).then(setIssues),
        api.getMapData().then(setMapData),
      ]);
    }).finally(() => setLoading(false));
  }, [districtUID]);

  const exportPDF = useCallback(async () => {
    if (!reportRef.current) return;
    setExporting(true);
    try {
      // Capture each section separately for clean page breaks
      const sections = reportRef.current.querySelectorAll<HTMLElement>('[data-pdf-section]');
      const pdf = new jsPDF('p', 'mm', 'a4');
      const pdfWidth = pdf.internal.pageSize.getWidth();
      const pdfHeight = pdf.internal.pageSize.getHeight();
      const margin = 10;
      const usableWidth = pdfWidth - margin * 2;
      let currentY = margin;
      let firstPage = true;

      for (const section of sections) {
        const canvas = await html2canvas(section, {
          scale: 2,
          useCORS: true,
          logging: false,
          backgroundColor: '#ffffff',
        });

        const imgWidth = usableWidth;
        const imgHeight = (canvas.height * imgWidth) / canvas.width;

        // If section doesn't fit on current page, add new page
        if (!firstPage && currentY + imgHeight > pdfHeight - margin) {
          pdf.addPage();
          currentY = margin;
        }

        const imgData = canvas.toDataURL('image/png');
        pdf.addImage(imgData, 'PNG', margin, currentY, imgWidth, imgHeight);
        currentY += imgHeight + 4;
        firstPage = false;
      }

      pdf.save(`rapport_${districtName}_${new Date().toISOString().slice(0, 10)}.pdf`);
    } finally {
      setExporting(false);
    }
  }, [districtName]);

  const mainCommodites = commodites.filter(c => ['energie', 'eau_pts_critiques'].includes(c.indicator));
  const energyTypes = commodites.filter(c => ['energie_solaire', 'energie_reseau', 'energie_generateur'].includes(c.indicator));

  const rhPieData = rhSummary ? [
    { name: 'Fonctionnaires', value: rhSummary.total_fonc, fill: '#3b82f6' },
    { name: 'Contractuels', value: rhSummary.total_contr, fill: '#f59e0b' },
    { name: 'Benevoles', value: rhSummary.total_benev, fill: '#22c55e' },
  ].filter(d => d.value > 0) : [];

  const pctColor = (v: number) => v >= 80 ? '#16a34a' : v >= 50 ? '#ca8a04' : '#dc2626';

  if (loading) return <div className="p-8 text-gray-400">Chargement du rapport...</div>;

  const pageStyle: React.CSSProperties = {
    maxWidth: '210mm',
    margin: '0 auto',
    padding: '15mm',
    background: 'white',
    boxShadow: '0 0 20px rgba(0,0,0,0.08)',
    fontFamily: 'Helvetica, Arial, sans-serif',
  };

  return (
    <div>
      {/* Controls */}
      <div className="flex items-center justify-between mb-4 print:hidden" style={{ maxWidth: '210mm', margin: '0 auto' }}>
        <button onClick={() => navigate(-1)} className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800">
          <ArrowLeft size={16} /> Retour
        </button>
        <button
          onClick={exportPDF}
          disabled={exporting}
          className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 text-sm font-medium"
        >
          <Download size={16} />
          {exporting ? 'Generation...' : 'Telecharger PDF'}
        </button>
      </div>

      {/* Report */}
      <div ref={reportRef} style={pageStyle}>

        {/* Section: Header + KPIs */}
        <div data-pdf-section>
          <div style={{ borderBottom: '3px solid #1e40af', paddingBottom: '12px', marginBottom: '20px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
              <div>
                <h1 style={{ fontSize: '24px', fontWeight: 800, color: '#1e3a5f', margin: 0 }}>Rapport ISS</h1>
                <h2 style={{ fontSize: '18px', fontWeight: 600, color: '#374151', margin: '4px 0 0' }}>{districtName}</h2>
              </div>
              <div style={{ textAlign: 'right', fontSize: '11px', color: '#6b7280' }}>
                <div>Informations des Structures Sanitaires</div>
                <div>Genere le {new Date().toLocaleDateString('fr-FR')}</div>
              </div>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: '12px', marginBottom: '24px' }}>
            <KpiBox label="Score qualite" value={qualitySummary ? qualitySummary.avg_score.toFixed(1) : '-'} color={pctColor(qualitySummary?.avg_score ?? 0)} sub={`${qualitySummary?.n_structures ?? 0} structures`} />
            <KpiBox label="Rapportage" value={reporting ? `${reporting.pct.toFixed(1)}%` : '-'} color={pctColor(reporting?.pct ?? 0)} sub={`${reporting?.n_reported ?? 0} / ${reporting?.n_expected ?? 0}`} />
            <KpiBox label="Effectif RH" value={String(rhSummary?.total_effectif ?? 0)} color="#1e3a5f" sub={`${rhSummary?.ratio_med_per_structure?.toFixed(2) ?? '-'} med/struct`} />
            <KpiBox label="Structures" value={String(qualitySummary?.n_structures ?? 0)} color="#1e3a5f" sub={`${qualitySummary?.n_error ?? 0} err, ${qualitySummary?.n_warning ?? 0} avert.`} />
          </div>
        </div>

        {/* Section: Map + Commodites */}
        <div data-pdf-section>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '24px' }}>
            <div>
              <SectionTitle>Localisation</SectionTitle>
              <div style={{ height: '200px', border: '1px solid #e5e7eb', borderRadius: '6px', overflow: 'hidden' }}>
                {mapData && mapData.features.length > 0 ? (
                  <MapContainer
                    center={[10.5, -11.8]}
                    zoom={7}
                    style={{ height: '100%', width: '100%', background: '#ffffff' }}
                    scrollWheelZoom={false}
                    dragging={false}
                    zoomControl={false}
                    attributionControl={false}
                  >
                    <GeoJSON
                      data={mapData as unknown as GeoJSON.FeatureCollection}
                      style={(feature?: Feature<Geometry>) => {
                        const isTarget = feature?.properties?.district_name === districtName;
                        return {
                          fillColor: isTarget ? '#2563eb' : '#e5e7eb',
                          weight: isTarget ? 2 : 0.5,
                          color: isTarget ? '#1d4ed8' : '#9ca3af',
                          fillOpacity: isTarget ? 0.7 : 0.3,
                        } as PathOptions;
                      }}
                    />
                  </MapContainer>
                ) : (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: '#9ca3af', fontSize: '12px' }}>Carte non disponible</div>
                )}
              </div>
            </div>

            <div>
              <SectionTitle>Commodites (WASH / Energie)</SectionTitle>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px' }}>
                {mainCommodites.map(c => (
                  <div key={c.indicator} style={{ background: '#f9fafb', borderRadius: '6px', padding: '10px', textAlign: 'center' }}>
                    <div style={{ fontSize: '10px', color: '#6b7280' }}>{c.indicator === 'energie' ? 'Energie' : 'Eau pts critiques'}</div>
                    <div style={{ fontSize: '22px', fontWeight: 700, color: pctColor(c.pct) }}>{c.pct.toFixed(0)}%</div>
                    <div style={{ fontSize: '9px', color: '#9ca3af' }}>{c.n_oui}/{c.n_total}</div>
                  </div>
                ))}
              </div>
              {energyTypes.length > 0 && (
                <table style={{ width: '100%', fontSize: '10px', marginTop: '8px', borderCollapse: 'collapse' }}>
                  <tbody>
                    {energyTypes.map(c => (
                      <tr key={c.indicator} style={{ borderBottom: '1px solid #f3f4f6' }}>
                        <td style={{ padding: '3px 0', color: '#6b7280' }}>{c.indicator.replace('energie_', '')}</td>
                        <td style={{ padding: '3px 0', textAlign: 'right', fontWeight: 600 }}>{c.pct.toFixed(0)}%</td>
                        <td style={{ padding: '3px 0', textAlign: 'right', color: '#9ca3af' }}>{c.n_oui}/{c.n_total}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>

        {/* Section: Plateau technique */}
        {plateau.length > 0 && (
          <div data-pdf-section style={{ marginBottom: '24px' }}>
            <SectionTitle>Plateau technique</SectionTitle>
            <ResponsiveContainer width="100%" height={Math.max(200, plateau.length * 28)}>
              <BarChart data={plateau} layout="vertical" margin={{ left: 140 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis type="number" domain={[0, 100]} unit="%" tick={{ fontSize: 9 }} />
                <YAxis type="category" dataKey="service_label" tick={{ fontSize: 9 }} width={130} />
                <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
                <Bar dataKey="pct" name="% disponibilite">
                  {plateau.map((d, i) => (
                    <Cell key={i} fill={d.pct >= 75 ? '#22c55e' : d.pct >= 50 ? '#eab308' : d.pct >= 25 ? '#f97316' : '#ef4444'} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}

        {/* Section: Services */}
        {services.length > 0 && (
          <div data-pdf-section style={{ marginBottom: '24px' }}>
            <SectionTitle>Services disponibles</SectionTitle>
            <ReportTable
              headers={['Service', 'Fonctionnel', 'Total', '%']}
              rows={services.map(s => [s.service_label, String(s.n_oui), String(s.n_total), `${s.pct_fonctionnel.toFixed(1)}%`])}
            />
          </div>
        )}

        {/* Section: Equipements */}
        {equipements.length > 0 && (
          <div data-pdf-section style={{ marginBottom: '24px' }}>
            <SectionTitle>Equipements</SectionTitle>
            <ReportTable
              headers={['Equipement', 'Total', 'Fonctionnel', '%']}
              rows={equipements.map(e => [e.label, String(e.sum_total), String(e.sum_fonct), `${e.pct_fonct.toFixed(1)}%`])}
            />
          </div>
        )}

        {/* Section: RH */}
        {rh.length > 0 && (
          <div data-pdf-section style={{ marginBottom: '24px' }}>
            <SectionTitle>Ressources humaines</SectionTitle>
            <div style={{ display: 'grid', gridTemplateColumns: '200px 1fr', gap: '16px' }}>
              {rhPieData.length > 0 && (
                <ResponsiveContainer width="100%" height={180}>
                  <PieChart>
                    <Pie data={rhPieData} dataKey="value" nameKey="name" cx="50%" cy="50%" innerRadius={35} outerRadius={65}>
                      {rhPieData.map((entry, i) => (
                        <Cell key={i} fill={entry.fill} />
                      ))}
                    </Pie>
                    <Tooltip />
                    <Legend wrapperStyle={{ fontSize: '9px' }} />
                  </PieChart>
                </ResponsiveContainer>
              )}
              <ReportTable
                headers={['Profil', 'Fonc.', 'Contr.', 'Benev.', 'Total']}
                rows={rh.map(r => [r.label, String(r.effectif_fonc), String(r.effectif_contr), String(r.effectif_benev), String(r.effectif_total)])}
              />
            </div>
          </div>
        )}

        {/* Section: Issues */}
        {issues && issues.data.length > 0 && (
          <div data-pdf-section style={{ marginBottom: '24px' }}>
            <SectionTitle>Problemes qualite ({issues.total} structures)</SectionTitle>
            <ReportTable
              headers={['Structure', 'Severite', 'Score', 'E/W/I']}
              rows={issues.data.map(item => [
                item.org_unit_name,
                item.worst_severity,
                String(item.score),
                `${item.n_error}/${item.n_warning}/${item.n_info}`,
              ])}
            />
          </div>
        )}

        {/* Footer */}
        <div data-pdf-section>
          <div style={{ borderTop: '1px solid #e5e7eb', paddingTop: '8px', fontSize: '9px', color: '#9ca3af', display: 'flex', justifyContent: 'space-between' }}>
            <span>ISS Dashboard — Programme DHIS2 Informations des Structures Sanitaires</span>
            <span>{new Date().toLocaleDateString('fr-FR')}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h3 style={{ fontSize: '13px', fontWeight: 700, color: '#1e3a5f', borderBottom: '2px solid #e5e7eb', paddingBottom: '4px', marginBottom: '10px' }}>
      {children}
    </h3>
  );
}

function KpiBox({ label, value, color, sub }: { label: string; value: string; color: string; sub: string }) {
  return (
    <div style={{ background: '#f9fafb', borderRadius: '8px', padding: '12px', textAlign: 'center', border: '1px solid #e5e7eb' }}>
      <div style={{ fontSize: '10px', color: '#6b7280', fontWeight: 500 }}>{label}</div>
      <div style={{ fontSize: '24px', fontWeight: 800, color, marginTop: '4px' }}>{value}</div>
      <div style={{ fontSize: '9px', color: '#9ca3af', marginTop: '2px' }}>{sub}</div>
    </div>
  );
}

function ReportTable({ headers, rows }: { headers: string[]; rows: string[][] }) {
  return (
    <table style={{ width: '100%', fontSize: '10px', borderCollapse: 'collapse' }}>
      <thead>
        <tr>
          {headers.map((h, i) => (
            <th key={i} style={{ background: '#1e3a5f', color: 'white', padding: '5px 8px', textAlign: i === 0 ? 'left' : 'center', fontWeight: 600, fontSize: '9px' }}>
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, j) => (
          <tr key={j} style={{ background: j % 2 === 0 ? '#f9fafb' : 'white' }}>
            {row.map((cell, i) => (
              <td key={i} style={{ padding: '4px 8px', textAlign: i === 0 ? 'left' : 'center', borderBottom: '1px solid #f3f4f6', color: '#374151' }}>
                {cell}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
