/** Message d'erreur lisible à partir d'une réponse HTTP en échec.
 *
 *  Un proxy placé devant l'API remplace souvent le corps d'un 5xx par sa propre
 *  page de maintenance HTML : l'afficher tel quel déversait des centaines de
 *  lignes de balises dans l'interface. On ne garde donc que le champ `error`
 *  d'une réponse JSON, ou un message générique quand le corps n'est pas de nous.
 */
export async function httpError(res: Response): Promise<Error> {
  const body = await res.text().catch(() => '');
  const trimmed = body.trim();

  if (trimmed.startsWith('{')) {
    try {
      const json = JSON.parse(trimmed) as { error?: string; message?: string };
      const msg = json.error ?? json.message;
      if (msg) return new Error(`API ${res.status} : ${msg}`);
    } catch {
      // corps JSON illisible : on retombe sur le message générique
    }
  }
  if (res.status === 503) {
    return new Error('API 503 : service momentanément indisponible (déploiement en cours, ou aucune synchronisation effectuée).');
  }
  if (res.status === 504) return new Error('API 504 : le serveur a mis trop de temps à répondre.');
  return new Error(`API ${res.status} : ${res.statusText || 'erreur serveur'}`);
}
