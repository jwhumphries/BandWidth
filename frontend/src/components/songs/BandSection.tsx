import type {BandLayer} from '../../lib/types';
import StatusBadge from './StatusBadge';

export default function BandSection({band}: {band: BandLayer}) {
  return (
    <section className="card bg-base-200 shadow">
      <div className="card-body">
        <h2 className="card-title">Band: {band.bandName}</h2>
        <p className="text-base-content/70 text-sm">
          Shared with your band — read-only here; edit it in the band view.
        </p>
        <div className="flex items-center gap-2">
          <span>Band status:</span>
          <StatusBadge status={band.status} />
        </div>
        {band.notes && <p className="whitespace-pre-wrap">{band.notes}</p>}
        <p className="text-base-content/70 text-sm">
          {band.rehearsalCount} rehearsals
          {band.lastRehearsedAt && <> · last on {band.lastRehearsedAt}</>}
        </p>
        {band.resources.length > 0 && (
          <ul className="flex flex-col gap-1">
            {band.resources.map(r => (
              <li key={r.id}>
                <a
                  className="link truncate"
                  href={r.url}
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {r.label || r.url}
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
