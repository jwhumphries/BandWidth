import {CircleCheck, Clock, RotateCcw} from 'lucide-react';
import {Link} from 'react-router';
import type {SongListItem} from '../../lib/types';
import StatusBadge from '../songs/StatusBadge';

export default function PracticeRow({
  song,
  linkTo,
  done,
  actionLabel,
  canAct,
  disabledReason,
  onToggle,
}: {
  song: SongListItem;
  linkTo: string;
  done: boolean;
  actionLabel: string;
  canAct: boolean;
  disabledReason?: string;
  onToggle: () => void;
}) {
  return (
    <li
      className={`group border-base-300/60 bg-base-100 hover:border-base-300 flex items-center gap-3 rounded-box border p-3 transition-all hover:shadow-md sm:gap-4 sm:p-4 ${
        done ? 'opacity-60' : ''
      }`}
    >
      <Link to={linkTo} className="min-w-0 flex-1">
        <span
          className={`font-display group-hover:text-primary block truncate text-base font-semibold transition-colors ${
            done ? 'line-through' : ''
          }`}
        >
          {song.title}
        </span>
        <span className="text-base-content/55 block truncate text-sm">
          {song.artist || '—'}
        </span>
      </Link>

      <div className="flex shrink-0 items-center gap-2 sm:gap-3">
        <StatusBadge status={song.status} />
        <span className="text-base-content/45 hidden w-28 items-center justify-end gap-1 font-mono text-xs sm:flex">
          <Clock className="size-3 shrink-0" />
          {song.lastPracticedAt || 'never'}
        </span>
        <button
          type="button"
          className={`btn btn-sm gap-1.5 ${done ? 'btn-ghost' : 'btn-primary'}`}
          onClick={onToggle}
          disabled={!canAct}
          title={canAct ? undefined : disabledReason}
        >
          {done ? (
            <RotateCcw className="size-4" />
          ) : (
            <CircleCheck className="size-4" />
          )}
          {done ? 'Undo' : actionLabel}
        </button>
      </div>
    </li>
  );
}
