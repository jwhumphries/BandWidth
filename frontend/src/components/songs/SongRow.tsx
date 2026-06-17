import {CircleCheck, Clock, Users} from 'lucide-react';
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
    <li className="group border-base-300/60 bg-base-100 hover:border-base-300 flex items-center gap-3 rounded-box border p-3 transition-all hover:shadow-md sm:gap-4 sm:p-4">
      <Link to={`/songs/${song.id}`} className="min-w-0 flex-1">
        <span className="font-display group-hover:text-primary block truncate text-base font-semibold transition-colors">
          {song.title}
        </span>
        <span className="text-base-content/55 block truncate text-sm">
          {song.artist || '—'}
        </span>
      </Link>

      <div className="flex shrink-0 items-center gap-2 sm:gap-3">
        {song.bandName && (
          <span className="border-base-300 text-base-content/70 hidden items-center gap-1 rounded-selector border px-2 py-0.5 text-xs font-medium md:inline-flex">
            <Users className="size-3" />
            {song.bandName}
          </span>
        )}
        <StatusBadge status={song.status} />
        <span className="text-base-content/45 hidden w-28 items-center justify-end gap-1 font-mono text-xs sm:flex">
          <Clock className="size-3 shrink-0" />
          {song.lastPracticedAt || 'never'}
        </span>
        <button
          className="btn btn-primary btn-sm gap-1.5"
          onClick={() => onPracticed(song.id, localToday())}
        >
          <CircleCheck className="size-4" />
          Practiced
        </button>
      </div>
    </li>
  );
}
