import {Link, useParams} from 'react-router';
import BandSettings from '../components/bands/BandSettings';
import InviteManager from '../components/bands/InviteManager';
import MemberList from '../components/bands/MemberList';
import {useBand} from '../hooks/bands';

export default function BandPage() {
  const {id: idParam} = useParams();
  const id = Number(idParam);
  const {data: band, isPending, isError, error, refetch} = useBand(id);

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
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-bold">{band.name}</h1>
      <MemberList band={band} />
      {isAdmin && <InviteManager bandId={band.id} />}
      {isAdmin && <BandSettings band={band} />}
    </div>
  );
}
