import { useEffect, useState, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, ChevronDown, ChevronRight, FileDown } from 'lucide-react';
import { api } from '../api/client';
import type { EventDetail } from '../types';
import ScoreBar from '../components/ScoreBar';
import SeverityBadge from '../components/SeverityBadge';
import MethodNote from '../components/MethodNote';

const SECTION_LABELS: Record<string, string> = {
  ISS_GEN: 'Informations generales',
  ISS_SVC: 'Services',
  ISS_EQ: 'Equipements',
  ISS_EQUI: 'Equipements',
  ISS_RH: 'Ressources humaines',
  ISS_RH_SPE: 'Ressources humaines (specialistes)',
  ISS_COMMO: 'Commodites',
  ISS_INFRA: 'Infrastructure',
  ISS_LAB: 'Laboratoire',
};

function getSectionLabel(prefix: string): string {
  if (!prefix) return 'Autres';
  // Try exact match first, then prefix match
  if (SECTION_LABELS[prefix]) return SECTION_LABELS[prefix];
  for (const [key, label] of Object.entries(SECTION_LABELS)) {
    if (prefix.startsWith(key)) return label;
  }
  return prefix;
}

export default function StructureDetail() {
  const { uid } = useParams<{ uid: string }>();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<EventDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [openSections, setOpenSections] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (!uid) return;
    setLoading(true);
    api.getEventDetail(uid)
      .then((d) => {
        setDetail(d);
        // Open all sections by default
        if (d?.values) {
          const sections = new Set(d.values.map(v => getSectionLabel(v.section_prefix)));
          setOpenSections(sections);
        }
      })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, [uid]);

  // Group values by section
  const groupedValues = useMemo(() => {
    if (!detail?.values) return [];
    const groups = new Map<string, typeof detail.values>();
    for (const v of detail.values) {
      const label = getSectionLabel(v.section_prefix);
      if (!groups.has(label)) groups.set(label, []);
      groups.get(label)!.push(v);
    }
    return Array.from(groups.entries());
  }, [detail]);

  const toggleSection = (section: string) => {
    setOpenSections(prev => {
      const next = new Set(prev);
      if (next.has(section)) next.delete(section);
      else next.add(section);
      return next;
    });
  };

  if (loading) return <div className="p-6 text-gray-500">Chargement...</div>;
  if (error) return <div className="p-6 text-red-500">Erreur : {error}</div>;
  if (!detail) return <div className="p-6 text-gray-500">Structure introuvable.</div>;

  const evt = detail.event;
  const q = detail.quality;

  return (
    <div className="space-y-4 max-w-4xl">
      {/* Back button + PDF export */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => navigate('/structures')}
          className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800"
        >
          <ArrowLeft size={16} />
          Retour aux structures
        </button>
        {uid && (
          <button
            onClick={() => api.exportStructurePDF(uid)}
            className="flex items-center gap-1 px-3 py-1.5 text-sm bg-red-600 text-white rounded hover:bg-red-700"
          >
            <FileDown size={16} />
            Exporter PDF
          </button>
        )}
      </div>

      {/* Header */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 sm:p-6">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
          <div>
            <h2 className="text-lg sm:text-xl font-bold text-gray-900">{evt.org_unit_name}</h2>
            <p className="text-sm text-gray-500 mt-1">{evt.district} — {evt.region}</p>
            <div className="flex flex-wrap gap-3 mt-2 text-xs text-gray-400">
              <span>Date : {evt.event_date?.slice(0, 10)}</span>
              <span>Statut : {evt.status}</span>
            </div>
          </div>
          <div className="flex flex-col items-end gap-1">
            {q && (
              <>
                <ScoreBar score={q.score} />
                <div className="flex gap-2 text-xs mt-1">
                  <span className="text-red-600">{q.n_error} erreurs</span>
                  <span className="text-yellow-600">{q.n_warning} avert.</span>
                  <span className="text-blue-600">{q.n_info} infos</span>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Quality issues */}
      {detail.issues && detail.issues.length > 0 && (
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <h3 className="font-semibold text-gray-800 mb-3">Problemes qualite ({detail.issues.length})</h3>
          <div className="space-y-2">
            {detail.issues.map((iss, i) => (
              <div key={i} className="flex gap-2 items-start text-sm">
                <SeverityBadge severity={iss.severity} />
                <div>
                  <span className="text-gray-500 text-xs mr-2">{iss.rule_code}</span>
                  <span className="text-gray-700">{iss.message}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Data values grouped by section */}
      {groupedValues.map(([section, values]) => {
        const isOpen = openSections.has(section);
        return (
          <div key={section} className="bg-white rounded-lg border border-gray-200">
            <button
              onClick={() => toggleSection(section)}
              className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-gray-50"
            >
              <h3 className="font-semibold text-gray-800 text-sm">
                {section} <span className="font-normal text-gray-400">({values.length})</span>
              </h3>
              {isOpen ? <ChevronDown size={16} className="text-gray-400" /> : <ChevronRight size={16} className="text-gray-400" />}
            </button>
            {isOpen && (
              <div className="border-t border-gray-100">
                <table className="w-full text-sm">
                  <tbody>
                    {values.map((v, i) => (
                      <tr key={i} className="border-b border-gray-50">
                        <td className="px-4 py-1.5 text-gray-500 w-1/2">{v.de_name}</td>
                        <td className="px-4 py-1.5 font-medium text-gray-800">{v.value || '-'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        );
      })}

      <MethodNote title="Methodologie - Fiche structure">
        <p>Cette fiche presente l'ensemble des donnees saisies pour une structure sanitaire dans le programme ISS DHIS2.</p>
        <p>Les donnees sont groupees par section : informations generales, services, equipements, ressources humaines, commodites, infrastructure, laboratoire.</p>
        <p>Le <strong>score qualite</strong> (0-100) est calcule selon les regles : -15/erreur, -5/avertissement, -1/info.</p>
      </MethodNote>
    </div>
  );
}
