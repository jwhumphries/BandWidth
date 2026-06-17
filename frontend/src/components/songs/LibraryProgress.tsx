import type {SongListItem, SongStatus} from '../../lib/types';

const order: Array<{status: SongStatus; label: string}> = [
  {status: 'not_learned', label: 'Not learned'},
  {status: 'learning', label: 'Learning'},
  {status: 'learned', label: 'Learned'},
  {status: 'nailed', label: 'Nailed'},
];

/** A segmented meter of how the visible songs spread across the four
    statuses — the practice-progress story at a glance. */
export default function LibraryProgress({songs}: {songs: SongListItem[]}) {
  const total = songs.length;
  if (total === 0) return null;

  const counts = songs.reduce<Record<SongStatus, number>>(
    (acc, s) => {
      acc[s.status] = (acc[s.status] ?? 0) + 1;
      return acc;
    },
    {not_learned: 0, learning: 0, learned: 0, nailed: 0},
  );

  return (
    <div className="border-base-300/60 bg-base-100 rounded-box border p-4">
      <div className="flex h-2.5 overflow-hidden rounded-selector">
        {order.map(({status}) =>
          counts[status] > 0 ? (
            <div
              key={status}
              className={`status-fill-${status}`}
              style={{
                width: `${(counts[status] / total) * 100}%`,
                backgroundColor: `var(--status-${status.replace('_', '-')})`,
              }}
              title={`${counts[status]} ${status}`}
            />
          ) : null,
        )}
      </div>
      <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1.5">
        {order.map(({status, label}) => (
          <span
            key={status}
            className="text-base-content/65 flex items-center gap-1.5 text-xs font-medium"
          >
            <span
              className="size-2 rounded-full"
              style={{
                backgroundColor: `var(--status-${status.replace('_', '-')})`,
              }}
            />
            <span className="font-mono text-base-content/90">
              {counts[status]}
            </span>
            {label}
          </span>
        ))}
      </div>
    </div>
  );
}
