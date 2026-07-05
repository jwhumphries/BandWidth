import {Navigate, Outlet} from 'react-router';
import {useMe} from '../hooks/auth';

export default function RequireAdmin() {
  const {data, isPending} = useMe();
  if (isPending) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <span
          className="loading loading-spinner loading-lg"
          aria-label="Loading"
        />
      </div>
    );
  }
  if (!data?.isAdmin) {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}
