import {useState} from 'react';
import {Link} from 'react-router';
import StatusBadge from '../songs/StatusBadge';
import AddBandSongModal from './AddBandSongModal';
import {useBandSongs} from '../../hooks/bandsongs';

export default function BandSongList({
  bandId,
  canEdit,
}: {
  bandId: number;
  canEdit: boolean;
}) {
  const {data: songs = []} = useBandSongs(bandId);
  const [adding, setAdding] = useState(false);

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <div className="flex items-center justify-between">
          <h2 className="card-title">Songs</h2>
          {canEdit && (
            <button
              className="btn btn-primary btn-sm"
              onClick={() => setAdding(true)}
            >
              Add song
            </button>
          )}
        </div>
        {songs.length === 0 ? (
          <p className="text-base-content/60 text-sm">No band songs yet.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {songs.map(song => (
              <li key={song.id}>
                <Link
                  to={`/bands/${bandId}/songs/${song.id}`}
                  className="bg-base-200 flex items-center gap-3 rounded-box p-3"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-semibold">
                      {song.title}
                    </span>
                    <span className="text-base-content/60 block truncate text-sm">
                      {song.artist || '—'}
                    </span>
                  </span>
                  <StatusBadge status={song.status} />
                </Link>
              </li>
            ))}
          </ul>
        )}
        <AddBandSongModal
          bandId={bandId}
          open={adding}
          onClose={() => setAdding(false)}
        />
      </div>
    </section>
  );
}
