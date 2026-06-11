import {useMe} from '../hooks/auth';

export default function HomePage() {
  const {data: user} = useMe();
  return (
    <div className="hero bg-base-100 rounded-box py-12">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-4xl font-bold">Welcome, {user?.username}</h1>
          <p className="py-4">Your songs will live here soon.</p>
        </div>
      </div>
    </div>
  );
}
