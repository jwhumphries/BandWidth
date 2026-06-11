import {Navigate, Outlet} from 'react-router';
import {useMe} from '../hooks/auth';

export default function RequireAuth() {
  const {isPending, isError} = useMe();
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
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}
