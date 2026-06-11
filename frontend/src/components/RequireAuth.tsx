import {Navigate, Outlet} from 'react-router';
import {useMe} from '../hooks/auth';
import {ApiError} from '../lib/api';

export default function RequireAuth() {
  const {isPending, isError, error, refetch} = useMe();
  if (isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <span
          className="loading loading-spinner loading-lg"
          aria-label="Loading"
        />
      </div>
    );
  }
  if (isError) {
    if (
      error instanceof ApiError &&
      (error.status === 401 || error.status === 403)
    ) {
      return <Navigate to="/login" replace />;
    }
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4">
        <p>Could not reach the server.</p>
        <button className="btn btn-primary" onClick={() => void refetch()}>
          Retry
        </button>
      </div>
    );
  }
  return <Outlet />;
}
