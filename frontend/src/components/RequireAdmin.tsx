import {Navigate, Outlet} from 'react-router';
import {useMe} from '../hooks/auth';

export default function RequireAdmin() {
  const {data, isPending} = useMe();
  if (isPending) {
    return (
      <div className="flex justify-center py-12">
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
