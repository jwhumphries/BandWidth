import {useNavigate, useParams} from 'react-router';
import {useJoinByLink} from '../hooks/invites';

export default function JoinPage() {
  const {token} = useParams();
  const navigate = useNavigate();
  const join = useJoinByLink();

  if (!token) {
    return <p className="py-12 text-center">Invalid invite link.</p>;
  }

  return (
    <div className="flex flex-col items-center gap-4 py-12">
      <h1 className="text-2xl font-bold">Join band</h1>
      <p>You have been invited to join a band.</p>
      {join.error && (
        <div role="alert" className="alert alert-error">
          {join.error.message}
        </div>
      )}
      <button
        className="btn btn-primary"
        disabled={join.isPending}
        onClick={() =>
          join.mutate(token, {
            onSuccess: ({bandId}) => void navigate(`/bands/${bandId}`),
          })
        }
      >
        Join
      </button>
    </div>
  );
}
