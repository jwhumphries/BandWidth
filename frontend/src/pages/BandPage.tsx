import {ArrowLeft} from 'lucide-react';
import {Link, useParams} from 'react-router';
import BandFolderSidebar from '../components/bands/BandFolderSidebar';
import BandSettings from '../components/bands/BandSettings';
import BandSongList from '../components/bands/BandSongList';
import InviteManager from '../components/bands/InviteManager';
import MemberList from '../components/bands/MemberList';
import {useBand} from '../hooks/bands';
import {useBandFolders} from '../hooks/bandfolders';
import {useFolderSelection} from '../lib/folderSelection';

export default function BandPage() {
  const {id: idParam} = useParams();
  const id = Number(idParam);
  const {data: band, isPending, isError, error, refetch} = useBand(id);
  const {data: folders} = useBandFolders(id);
  // Scoped per band, so switching bands does not carry a folder across.
  const [folderId, setFolderId] = useFolderSelection(`band:${id}`, folders);

  if (isPending) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner" aria-label="Loading" />
      </div>
    );
  }
  if (isError || !band) {
    return (
      <div className="flex flex-col items-center gap-4 py-12">
        <p>{error?.message ?? 'Could not load this band.'}</p>
        <div className="flex gap-2">
          <button className="btn" onClick={() => void refetch()}>
            Retry
          </button>
          <Link className="btn btn-ghost" to="/bands">
            Back to bands
          </Link>
        </div>
      </div>
    );
  }

  const isAdmin = band.myRole === 'admin';
  const canEdit = band.myRole !== 'viewer';
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <Link
          to="/bands"
          className="text-base-content/60 hover:text-base-content inline-flex w-fit items-center gap-1.5 text-sm font-medium transition-colors"
        >
          <ArrowLeft className="size-4" />
          Bands
        </Link>
        <h1 className="font-display text-3xl font-bold tracking-tight">
          {band.name}
        </h1>
      </div>
      <div className="flex flex-col gap-6 sm:flex-row">
        <BandFolderSidebar
          bandId={band.id}
          canEdit={canEdit}
          selectedId={folderId}
          onSelect={setFolderId}
        />
        <div className="flex-1">
          <BandSongList
            bandId={band.id}
            canEdit={canEdit}
            folderId={folderId}
          />
        </div>
      </div>
      <MemberList band={band} />
      {isAdmin && <InviteManager bandId={band.id} />}
      {isAdmin && <BandSettings band={band} />}
    </div>
  );
}
