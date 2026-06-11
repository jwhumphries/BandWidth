import {Link} from 'react-router';
import {localToday} from '../../lib/dates';
import type {SongListItem} from '../../lib/types';
import StatusBadge from './StatusBadge';

export default function SongRow({
  song,
  onPracticed,
}: {
  song: SongListItem;
  onPracticed: (id: number, date: string) => void;
}) {
  return (
    <li className="bg-base-100 flex items-center gap-3 rounded-box p-3 shadow-sm">
      <Link to={`/songs/${song.id}`} className="min-w-0 flex-1">
        <span className="block truncate font-semibold">{song.title}</span>
        <span className="text-base-content/60 block truncate text-sm">
          {song.artist || '—'}
        </span>
      </Link>
      <StatusBadge status={song.status} />
      <span className="text-base-content/60 hidden text-sm sm:block">
        {song.lastPracticedAt || 'Never practiced'}
      </span>
      <button
        className="btn btn-sm btn-outline"
        onClick={() => onPracticed(song.id, localToday())}
      >
        Practiced
      </button>
    </li>
  );
}
