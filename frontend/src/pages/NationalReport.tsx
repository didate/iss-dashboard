import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { ArrowLeft, Download } from 'lucide-react';
import html2canvas from 'html2canvas';
import { jsPDF } from 'jspdf';
import { api } from '../api/client';
import type {
  Summary, UsageRecensement, UsageService, UsageEquipement, UsageRH,
  UsageCommodite, RHSummaryResult, ReportingRate, ServiceMatrixRow, Filters,
} from '../types';

// --- Helpers ---


// --- Sub-components ---

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <h2 style={{ fontSize: '16px', fontWeight: 800, color: '#1e3a5f', borderBottom: '3px solid #1e40af', paddingBottom: '6px', marginBottom: '14px', marginTop: '8px' }}>
      {children}
    </h2>
  );
}

function SubTitle({ children }: { children: React.ReactNode }) {
  return (
    <h4 style={{ fontSize: '11px', fontWeight: 600, color: '#374151', marginBottom: '8px', marginTop: '14px' }}>
      {children}
    </h4>
  );
}

function ReportTable({ headers, rows }: { headers: string[]; rows: string[][] }) {
  return (
    <table style={{ width: '100%', fontSize: '9px', borderCollapse: 'collapse' }}>
      <thead>
        <tr>
          {headers.map((h, i) => (
            <th key={i} style={{ background: '#1e3a5f', color: 'white', padding: '4px 6px', textAlign: i === 0 ? 'left' : 'center', fontWeight: 600, fontSize: '8px' }}>
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, j) => (
          <tr key={j} style={{ background: j % 2 === 0 ? '#f9fafb' : 'white' }}>
            {row.map((cell, i) => (
              <td key={i} style={{ padding: '3px 6px', textAlign: i === 0 ? 'left' : 'center', borderBottom: '1px solid #f3f4f6', color: '#374151' }}>
                {cell}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function KpiBox({ label, value, sub, color = '#1e3a5f' }: { label: string; value: string; sub?: string; color?: string }) {
  return (
    <div style={{ background: '#f9fafb', borderRadius: '8px', padding: '12px', textAlign: 'center', border: '1px solid #e5e7eb' }}>
      <div style={{ fontSize: '10px', color: '#6b7280', fontWeight: 500 }}>{label}</div>
      <div style={{ fontSize: '22px', fontWeight: 800, color, marginTop: '4px' }}>{value}</div>
      {sub && <div style={{ fontSize: '9px', color: '#9ca3af', marginTop: '2px' }}>{sub}</div>}
    </div>
  );
}


const pctColor = (v: number) => v >= 80 ? '#16a34a' : v >= 50 ? '#ca8a04' : '#dc2626';

// --- Main Component ---

export default function NationalReport() {
  const navigate = useNavigate();
  const reportRef = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);

  const [filters, setFilters] = useState<Filters | null>(null);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [recensement, setRecensement] = useState<UsageRecensement[]>([]);
  const [recensementStatut, setRecensementStatut] = useState<UsageRecensement[]>([]);
  const [reportingGlobal, setReportingGlobal] = useState<ReportingRate | null>(null);
  const [services, setServices] = useState<UsageService[]>([]);
  const [serviceMatrix, setServiceMatrix] = useState<ServiceMatrixRow[]>([]);
  const [equipements, setEquipements] = useState<UsageEquipement[]>([]);
  const [rh, setRH] = useState<UsageRH[]>([]);
  const [rhSummary, setRHSummary] = useState<RHSummaryResult | null>(null);
  // Per-district commodites for regional aggregation
  const [commoditesAll, setCommoditesAll] = useState<UsageCommodite[]>([]);
  const [equipementsAll, setEquipementsAll] = useState<UsageEquipement[]>([]);
  const [rhAll, setRHAll] = useState<UsageRH[]>([]);
  const [reportingRegion, setReportingRegion] = useState<ReportingRate[]>([]);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api.getFilters().then(setFilters),
      api.getSummary().then(setSummary),
      api.getUsageRecensement('district').then(d => setRecensement(d ?? [])),
      api.getUsageRecensement('statut_juridique').then(d => setRecensementStatut(d ?? [])),
      api.getReportingRate('global').then(r => {
        const g = (r ?? []).find(x => x.key === 'all');
        if (g) setReportingGlobal(g);
      }),
      api.getUsageServices('').then(d => setServices(d ?? [])),
      api.getServiceMatrix().then(d => setServiceMatrix(d ?? [])),
      api.getUsageEquipements('all', '').then(d => setEquipements(d ?? [])),
      api.getUsageRH('').then(d => setRH(d ?? [])),
      api.getRHSummary('').then(setRHSummary),
      api.getReportingRate('region').then(d => setReportingRegion(d ?? [])),
    ]).finally(() => setLoading(false));
  }, []);

  // Fetch per-district data for regional aggregation once filters load
  useEffect(() => {
    if (!filters?.districts) return;
    Promise.all(
      filters.districts.map(d =>
        Promise.all([
          api.getUsageCommodites(d).then(data => (data ?? []).map(c => ({ ...c, district: d }))),
          api.getUsageEquipements('all', d).then(data => (data ?? []).map(e => ({ ...e, district: d }))),
          api.getUsageRH(d).then(data => (data ?? []).map(r => ({ ...r, district: d }))),
        ])
      )
    ).then(results => {
      setCommoditesAll(results.flatMap(r => r[0]));
      setEquipementsAll(results.flatMap(r => r[1]));
      setRHAll(results.flatMap(r => r[2]));
    });
  }, [filters]);

  const districtRegions = filters?.district_regions ?? {};
  const regions = filters?.regions ?? [];

  // --- Aggregation helpers ---

  // Commodites by region
  const commoditesByRegion = useCallback((indicator: string) => {
    const regionData: Record<string, { oui: number; total: number }> = {};
    for (const c of commoditesAll) {
      if (c.indicator !== indicator) continue;
      const region = districtRegions[c.district] || 'Inconnu';
      if (!regionData[region]) regionData[region] = { oui: 0, total: 0 };
      regionData[region].oui += c.n_oui;
      regionData[region].total += c.n_total;
    }
    return regions.map(r => ({
      name: r,
      n: regionData[r]?.oui ?? 0,
      pct: regionData[r] && regionData[r].total > 0 ? (regionData[r].oui / regionData[r].total * 100) : 0,
    }));
  }, [commoditesAll, districtRegions, regions]);

  // Equipment avg per structure by region
  const equipAvgByRegion = useCallback((filterFn: (e: UsageEquipement) => boolean) => {
    const regionData: Record<string, { total: number; nStruct: number }> = {};
    for (const e of equipementsAll) {
      if (!filterFn(e)) continue;
      const region = districtRegions[e.district] || 'Inconnu';
      if (!regionData[region]) regionData[region] = { total: 0, nStruct: 0 };
      regionData[region].total += e.sum_total;
    }
    // Get structure count per region from recensement
    const recByRegion: Record<string, number> = {};
    for (const r of recensement) {
      const region = districtRegions[r.label];
      if (region) {
        recByRegion[region] = (recByRegion[region] || 0) + r.n_structures;
      }
    }
    return regions.map(r => ({
      name: r,
      avg: recByRegion[r] && recByRegion[r] > 0 ? (regionData[r]?.total ?? 0) / recByRegion[r] : 0,
    }));
  }, [equipementsAll, districtRegions, regions, recensement]);

  // RH avg per structure by region
  const rhAvgByRegion = useCallback((filterFn: (r: UsageRH) => boolean) => {
    const regionData: Record<string, number> = {};
    for (const r of rhAll) {
      if (!filterFn(r)) continue;
      const region = districtRegions[r.district] || 'Inconnu';
      regionData[region] = (regionData[region] || 0) + r.effectif_total;
    }
    const recByRegion: Record<string, number> = {};
    for (const r of recensement) {
      const region = districtRegions[r.label];
      if (region) recByRegion[region] = (recByRegion[region] || 0) + r.n_structures;
    }
    return regions.map(r => ({
      name: r,
      avg: recByRegion[r] && recByRegion[r] > 0 ? (regionData[r] ?? 0) / recByRegion[r] : 0,
    }));
  }, [rhAll, districtRegions, regions, recensement]);

  // --- PDF Export ---
  const exportPDF = useCallback(async () => {
    if (!reportRef.current) return;
    setExporting(true);
    try {
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
          scale: 2, useCORS: true, logging: false, backgroundColor: '#ffffff',
        });
        const imgWidth = usableWidth;
        const imgHeight = (canvas.height * imgWidth) / canvas.width;

        if (!firstPage && currentY + imgHeight > pdfHeight - margin) {
          pdf.addPage();
          currentY = margin;
        }
        pdf.addImage(canvas.toDataURL('image/png'), 'PNG', margin, currentY, imgWidth, imgHeight);
        currentY += imgHeight + 4;
        firstPage = false;
      }
      pdf.save(`rapport_national_iss_${new Date().toISOString().slice(0, 10)}.pdf`);
    } finally {
      setExporting(false);
    }
  }, []);

  // Regional summary table data
  const regionalSummary = useMemo(() => {
    if (!regions.length || !recensement.length) return [];
    const recByRegion: Record<string, { total: number; oper: number }> = {};
    for (const r of recensement) {
      const region = districtRegions[r.label];
      if (!region) continue;
      if (!recByRegion[region]) recByRegion[region] = { total: 0, oper: 0 };
      recByRegion[region].total += r.n_structures;
      recByRegion[region].oper += r.n_operationnel;
    }
    const repByRegion = new Map(reportingRegion.map(r => [r.key, r]));
    const energieByRegion = new Map(commoditesByRegion('energie').map(d => [d.name, d]));
    const eauByRegion = new Map(commoditesByRegion('eau_pts_critiques').map(d => [d.name, d]));

    const medByRegion: Record<string, number> = {};
    for (const r of rhAll) {
      if (!r.profil_code?.includes('MED')) continue;
      const region = districtRegions[r.district] || 'Inconnu';
      medByRegion[region] = (medByRegion[region] || 0) + r.effectif_total;
    }

    return regions.map(r => {
      const rec = recByRegion[r] || { total: 0, oper: 0 };
      const rep = repByRegion.get(r);
      const energie = energieByRegion.get(r);
      const eau = eauByRegion.get(r);
      const medRatio = rec.total > 0 ? (medByRegion[r] || 0) / rec.total : 0;
      return {
        region: r,
        structures: rec.total,
        operationnel: rec.oper,
        rapportage: rep?.pct ?? 0,
        energiePct: energie?.pct ?? 0,
        eauPct: eau?.pct ?? 0,
        medParStruct: medRatio,
      };
    });
  }, [regions, recensement, districtRegions, reportingRegion, commoditesByRegion, rhAll]);

  if (loading) return <div className="p-8 text-gray-400">Chargement du rapport national...</div>;

  const pageStyle: React.CSSProperties = {
    maxWidth: '210mm', margin: '0 auto', padding: '15mm',
    background: 'white', boxShadow: '0 0 20px rgba(0,0,0,0.08)',
    fontFamily: 'Helvetica, Arial, sans-serif',
  };

  // Water source indicators
  const waterIndicators = ['source_eau_FMH', 'source_eau_FME', 'source_eau_FEM', 'source_eau_réseau', 'source_eau_puit', 'source_eau_aucune'];
  const waterLabels: Record<string, string> = {
    source_eau_FMH: 'Forage (FMH)', source_eau_FME: 'Forage (FME)', source_eau_FEM: 'Forage (FME)',
    'source_eau_réseau': 'Reseau public', source_eau_puit: 'Puit/Puit ameliore', source_eau_aucune: 'Aucune',
  };
  const energyIndicators = ['energie_solaire', 'energie_reseau', 'energie_generateur'];
  const energyLabels: Record<string, string> = {
    energie_solaire: 'Solaire', energie_reseau: 'Reseau electrique', energie_generateur: 'Generateur',
  };

  // Water data by region (raw numbers)
  const waterChartData = regions.map(r => {
    const row: Record<string, unknown> = { name: r };
    for (const ind of waterIndicators) {
      const data = commoditesByRegion(ind);
      const found = data.find(d => d.name === r);
      row[waterLabels[ind] || ind] = found ? found.n : 0;
    }
    return row;
  });

  // Energy data by region (raw numbers)
  const energyChartData = regions.map(r => {
    const row: Record<string, unknown> = { name: r };
    for (const ind of energyIndicators) {
      const data = commoditesByRegion(ind);
      const found = data.find(d => d.name === r);
      row[energyLabels[ind]] = found ? found.n : 0;
    }
    return row;
  });

  // Services by region for chart
  const labServices = services.filter(s => s.service_code?.includes('LAB') || s.service_code?.includes('EXAM'));
  const nonLabServices = services.filter(s => !labServices.includes(s));

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

      <div ref={reportRef} style={pageStyle}>

        {/* 1. Page de garde */}
        <div data-pdf-section>
          <div style={{ textAlign: 'center', paddingTop: '20px', paddingBottom: '30px', borderBottom: '3px solid #1e40af' }}>
            <div style={{ fontSize: '11px', color: '#6b7280', textTransform: 'uppercase', letterSpacing: '2px' }}>Republique de Guinee — Ministere de la Sante</div>
            <div style={{ fontSize: '10px', color: '#9ca3af', marginTop: '2px' }}>BSD / SNIS</div>
            <h1 style={{ fontSize: '26px', fontWeight: 800, color: '#1e3a5f', margin: '20px 0 8px' }}>Rapport sur les Informations</h1>
            <h1 style={{ fontSize: '26px', fontWeight: 800, color: '#1e3a5f', margin: '0 0 8px' }}>des Structures Sanitaires</h1>
            <div style={{ fontSize: '13px', color: '#374151', marginTop: '12px' }}>Programme ISS — DHIS2</div>
            <div style={{ fontSize: '11px', color: '#6b7280', marginTop: '8px' }}>Genere le {new Date().toLocaleDateString('fr-FR')}</div>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px', marginTop: '24px' }}>
            <KpiBox label="Structures analysees" value={String(summary?.n_structures ?? 0)} sub="dans le programme ISS" />
            <KpiBox label="Structures operationnelles" value={String(summary?.n_operationnel ?? 0)} sub={`sur ${summary?.n_structures ?? 0}`} />
            <KpiBox label="Taux de rapportage" value={reportingGlobal ? `${reportingGlobal.pct.toFixed(1)}%` : '-'} color={pctColor(reportingGlobal?.pct ?? 0)} sub={`${reportingGlobal?.n_reported ?? 0} / ${reportingGlobal?.n_expected ?? 0}`} />
          </div>
        </div>

        {/* Tableau recapitulatif par region */}
        {regionalSummary.length > 0 && (
          <div data-pdf-section style={{ marginTop: '24px' }}>
            <SectionTitle>1. Synthese par region</SectionTitle>

            <ReportTable
              headers={['Region', 'Structures', 'Operationnel', 'Rapportage', 'Energie', 'Eau pts crit.', 'Med./struct']}
              rows={regionalSummary.map(r => [
                r.region,
                String(r.structures),
                String(r.operationnel),
                `${r.rapportage.toFixed(1)}%`,
                `${r.energiePct.toFixed(1)}%`,
                `${r.eauPct.toFixed(1)}%`,
                r.medParStruct.toFixed(2),
              ])}
            />
          </div>
        )}

        {/* 2. Repartition des structures */}
        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SectionTitle>2. Repartition des structures sanitaires</SectionTitle>
          <SubTitle>Par district</SubTitle>
          <ReportTable
            headers={['District', 'Total', 'Operationnel', 'Non operationnel', 'Ferme temp.']}
            rows={recensement.map(r => [r.label, String(r.n_structures), String(r.n_operationnel), String(r.n_non_operationnel), String(r.n_ferme_temp)])}
          />
        </div>

        {recensementStatut.length > 0 && (
          <div data-pdf-section style={{ marginTop: '24px' }}>
            <SubTitle>Par statut juridique</SubTitle>
            <ReportTable
              headers={['Statut', 'Total', 'Operationnel', 'Non operationnel', 'Ferme temp.']}
              rows={recensementStatut.map(r => [r.label, String(r.n_structures), String(r.n_operationnel), String(r.n_non_operationnel), String(r.n_ferme_temp)])}
            />
          </div>
        )}

        {/* Commodites */}
        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SectionTitle>3. Commodites</SectionTitle>
          <SubTitle>Sources d'eau par region (nombre de structures)</SubTitle>
          {waterChartData.length > 0 && (
            <ReportTable
              headers={['Region', ...Object.values(waterLabels).filter((v, i, a) => a.indexOf(v) === i)]}
              rows={waterChartData.map(row => [
                row.name as string,
                ...Object.values(waterLabels).filter((v, i, a) => a.indexOf(v) === i).map(label =>
                  String(row[label] as number ?? 0)
                ),
              ])}
            />
          )}
        </div>

        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SubTitle>Sources d'energie par region (nombre de structures)</SubTitle>
          {energyChartData.length > 0 && (
            <ReportTable
              headers={['Region', 'Solaire', 'Reseau electrique', 'Generateur']}
              rows={energyChartData.map(row => [
                row.name as string,
                String(row['Solaire'] as number ?? 0),
                String(row['Reseau electrique'] as number ?? 0),
                String(row['Generateur'] as number ?? 0),
              ])}
            />
          )}
        </div>

        {/* 4. Equipements */}
        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SectionTitle>4. Equipements</SectionTitle>
        </div>
        {(() => {
          const categories = [
            { label: 'Motos', filter: (e: UsageEquipement) => e.equip_root?.includes('MOTO'), color: '#3b82f6' },
            { label: 'Vehicules', filter: (e: UsageEquipement) => e.equip_root?.includes('VEHIC') || e.equip_root?.includes('VOITURE'), color: '#f97316' },
            { label: 'Lits', filter: (e: UsageEquipement) => e.equip_root?.includes('LIT'), color: '#10b981' },
            { label: 'Porte-vaccins', filter: (e: UsageEquipement) => e.equip_root?.includes('PORTE_VACC'), color: '#8b5cf6' },
            { label: 'Glacieres', filter: (e: UsageEquipement) => e.equip_root?.includes('GLACIERE'), color: '#ef4444' },
            { label: 'Congelateurs', filter: (e: UsageEquipement) => e.equip_root?.includes('CONGELATEUR'), color: '#06b6d4' },
          ];
          return categories.map(cat => {
            const chartData = equipAvgByRegion(cat.filter).filter(d => d.avg > 0 || true);
            return (
              <div key={cat.label} data-pdf-section style={{ marginTop: '24px' }}>
                <SubTitle>Nombre moyen de {cat.label.toLowerCase()} par structure par region</SubTitle>
                <ResponsiveContainer width="100%" height={Math.max(180, chartData.length * 28)}>
                  <BarChart data={chartData} layout="vertical" margin={{ left: 100 }}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis type="number" tick={{ fontSize: 8 }} />
                    <YAxis type="category" dataKey="name" tick={{ fontSize: 8 }} width={90} />
                    <Tooltip formatter={(v: number) => v.toFixed(2)} />
                    <Bar dataKey="avg" name={cat.label} fill={cat.color} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            );
          });
        })()}

        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SubTitle>Detail par type (national)</SubTitle>
          <ReportTable
            headers={['Equipement', 'Total national', 'Fonctionnel', '% Fonctionnel']}
            rows={equipements.map(e => [e.label, String(e.sum_total), String(e.sum_fonct), `${e.pct_fonct.toFixed(1)}%`])}
          />
        </div>

        {/* 6. Ressources humaines */}
        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SectionTitle>5. Ressources humaines</SectionTitle>
          {rhSummary && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: '10px', marginBottom: '16px' }}>
              <KpiBox label="Effectif total" value={rhSummary.total_effectif.toLocaleString('fr-FR')} />
              <KpiBox label="Fonctionnaires" value={rhSummary.total_fonc.toLocaleString('fr-FR')} color="#3b82f6" />
              <KpiBox label="Contractuels" value={rhSummary.total_contr.toLocaleString('fr-FR')} color="#f59e0b" />
              <KpiBox label="Benevoles" value={rhSummary.total_benev.toLocaleString('fr-FR')} color="#22c55e" />
            </div>
          )}

          <SubTitle>Nombre moyen par structure par region</SubTitle>
          {(() => {
            const profiles = [
              { label: 'Medecins', filter: (r: UsageRH) => r.profil_code?.includes('MED') },
              { label: 'Infirmiers', filter: (r: UsageRH) => r.profil_code?.includes('INFIRMIER') },
              { label: 'Sages-femmes', filter: (r: UsageRH) => r.profil_code?.includes('SAGE') },
              { label: 'ATS', filter: (r: UsageRH) => r.profil_code?.includes('ATS') || r.profil_code?.includes('AGENT_TECH') },
              { label: 'Pharmaciens', filter: (r: UsageRH) => r.profil_code?.includes('PHARMA') },
            ];
            const rows = regions.map(r => {
              const cols = [r];
              for (const p of profiles) {
                const data = rhAvgByRegion(p.filter);
                const found = data.find(d => d.name === r);
                cols.push(found ? found.avg.toFixed(2) : '0');
              }
              return cols;
            });
            return (
              <ReportTable
                headers={['Region', ...profiles.map(p => p.label)]}
                rows={rows}
              />
            );
          })()}

          <SubTitle>Effectifs nationaux par profil</SubTitle>
          <ReportTable
            headers={['Profil', 'Fonctionnaires', 'Contractuels', 'Benevoles', 'Total']}
            rows={rh.map(r => [r.label, String(r.effectif_fonc), String(r.effectif_contr), String(r.effectif_benev), String(r.effectif_total)])}
          />
        </div>

        {/* 7. Services offerts */}
        <div data-pdf-section style={{ marginTop: '24px' }}>
          <SectionTitle>6. Services offerts</SectionTitle>
          <ReportTable
            headers={['Service', 'Nombre de structures']}
            rows={nonLabServices.map(s => [s.service_label, String(s.n_oui)])}
          />
        </div>

        {/* 7b. Service matrix */}
        {serviceMatrix.length > 0 && (
          <div data-pdf-section style={{ marginTop: '24px' }}>
            <SubTitle>Matrice services x districts</SubTitle>
            <div style={{ overflowX: 'auto' }}>
              <table style={{ fontSize: '7px', borderCollapse: 'collapse', width: '100%' }}>
                <thead>
                  <tr>
                    <th style={{ background: '#1e3a5f', color: 'white', padding: '3px 4px', textAlign: 'left', fontSize: '7px', position: 'sticky', left: 0 }}>Service</th>
                    <th style={{ background: '#1e3a5f', color: 'white', padding: '3px 4px', textAlign: 'center', fontSize: '7px' }}>Global</th>
                    {filters?.districts.map(d => (
                      <th key={d} style={{ background: '#1e3a5f', color: 'white', padding: '2px', textAlign: 'center', fontSize: '6px', writingMode: 'vertical-rl', maxHeight: '80px' }}>
                        {d}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {serviceMatrix.map((row, j) => (
                    <tr key={row.service_code} style={{ background: j % 2 === 0 ? '#f9fafb' : 'white' }}>
                      <td style={{ padding: '2px 4px', borderBottom: '1px solid #f3f4f6', fontWeight: 500 }}>{row.service_label}</td>
                      <td style={{ padding: '2px 4px', textAlign: 'center', borderBottom: '1px solid #f3f4f6', fontWeight: 600 }}>{row.overall.toFixed(0)}</td>
                      {filters?.districts.map(d => {
                        const val = row.districts[d] ?? 0;
                        const bg = val >= 75 ? '#dcfce7' : val >= 50 ? '#fef9c3' : val > 0 ? '#fee2e2' : '';
                        return (
                          <td key={d} style={{ padding: '2px', textAlign: 'center', borderBottom: '1px solid #f3f4f6', background: bg }}>
                            {val > 0 ? val.toFixed(0) : '-'}
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* 8. Laboratoire / Imagerie */}
        {labServices.length > 0 && (
          <div data-pdf-section style={{ marginTop: '24px' }}>
            <SectionTitle>7. Laboratoire et imagerie</SectionTitle>

            <ReportTable
              headers={['Examen / Service', 'Nombre de structures']}
              rows={labServices.map(s => [s.service_label, String(s.n_oui)])}
            />
          </div>
        )}

        {/* Footer */}
        <div data-pdf-section>
          <div style={{ borderTop: '2px solid #1e40af', paddingTop: '8px', marginTop: '24px', fontSize: '9px', color: '#9ca3af', display: 'flex', justifyContent: 'space-between' }}>
            <span>ISS Dashboard — Programme DHIS2 Informations des Structures Sanitaires</span>
            <span>{new Date().toLocaleDateString('fr-FR')}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
